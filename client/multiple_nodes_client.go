package client

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"sync"

	types "github.com/EspressoSystems/espresso-sequencer-go/types"
	common "github.com/EspressoSystems/espresso-sequencer-go/types/common"
)

var _ QueryService = (*MultipleNodesClient)(nil)
var _ SubmitAPI = (*MultipleNodesClient)(nil)

type MultipleNodesClient struct {
	nodes []*Client
}

func NewMultipleNodesClient(urls []string) *MultipleNodesClient {
	nodes := make([]*Client, len(urls))
	for i, url := range urls {
		nodes[i] = NewClient(url)
	}
	return &MultipleNodesClient{nodes: nodes}
}

func (c *MultipleNodesClient) FetchLatestBlockHeight(ctx context.Context) (uint64, error) {
	for _, node := range c.nodes {
		height, err := node.FetchLatestBlockHeight(ctx)
		if err == nil {
			return height, nil
		}
	}
	return 0, errors.New("no nodes available")
}

func (c *MultipleNodesClient) FetchHeaderByHeight(ctx context.Context, height uint64) (types.HeaderImpl, error) {
	var res types.HeaderImpl
	if err := c.getWithMajority(ctx, &res, "availability/header/%d", height); err != nil {
		return types.HeaderImpl{}, err
	}
	return res, nil
}

func (c *MultipleNodesClient) FetchRawHeaderByHeight(ctx context.Context, height uint64) (json.RawMessage, error) {
	return FetchWithMajority(ctx, c.nodes, func(node *Client) (json.RawMessage, error) {
		return node.FetchRawHeaderByHeight(ctx, height)
	})
}

func (c *MultipleNodesClient) FetchTransactionByHash(ctx context.Context, hash *types.TaggedBase64) (types.TransactionQueryData, error) {
	var res types.TransactionQueryData
	if err := c.getWithMajority(ctx, &res, "availability/transaction/hash/%s", hash.String()); err != nil {
		return types.TransactionQueryData{}, err
	}
	return res, nil
}

func (c *MultipleNodesClient) FetchHeadersByRange(ctx context.Context, from uint64, until uint64) ([]types.HeaderImpl, error) {
	var res []types.HeaderImpl
	if err := c.getWithMajority(ctx, &res, "availability/header/%d/%d", from, until); err != nil {
		return []types.HeaderImpl{}, err
	}
	return res, nil
}

func (c *MultipleNodesClient) getWithMajority(ctx context.Context, out any, format string, args ...any) error {
	body, err := FetchWithMajority(ctx, c.nodes, func(node *Client) (json.RawMessage, error) {
		return node.getRawMessage(ctx, format, args...)
	})
	if err != nil {
		return err
	}
	return json.Unmarshal(body, out)
}

func (c *MultipleNodesClient) FetchTransactionsInBlock(ctx context.Context, blockHeight uint64, namespace uint64) (TransactionsInBlock, error) {
	var res NamespaceResponse
	if err := c.getWithMajority(ctx, &res, "availability/block/%d/namespace/%d", blockHeight, namespace); err != nil {
		return TransactionsInBlock{}, err
	}

	if res.Transactions == nil {
		return TransactionsInBlock{}, fmt.Errorf("field transactions of type NamespaceResponse is required")
	}

	var txs []types.Bytes
	for i, tx := range *res.Transactions {
		if tx.Namespace != namespace {
			return TransactionsInBlock{}, fmt.Errorf("transaction %d has wrong namespace (%d, expected %d)", i, tx.Namespace, namespace)
		}
		txs = append(txs, tx.Payload)
	}

	if len(txs) > 0 && res.Proof == nil {
		return TransactionsInBlock{}, fmt.Errorf("field proof of type NamespaceResponse is required")
	}

	if res.Proof == nil {
		return TransactionsInBlock{}, nil
	}

	vidCommon, err := c.FetchVidCommonByHeight(ctx, blockHeight)
	if err != nil {
		return TransactionsInBlock{}, err
	}

	return TransactionsInBlock{Transactions: txs, Proof: *res.Proof, VidCommon: vidCommon}, nil
}

func (c *MultipleNodesClient) FetchVidCommonByHeight(ctx context.Context, blockHeight uint64) (common.VidCommon, error) {
	var res types.VidCommonQueryData
	if err := c.getWithMajority(ctx, &res, "availability/vid/common/%d", blockHeight); err != nil {
		return types.VidCommon{}, err
	}
	return res.Common, nil
}

func (c *MultipleNodesClient) SubmitTransaction(ctx context.Context, tx common.Transaction) (*common.TaggedBase64, error) {
	return c.nodes[0].SubmitTransaction(ctx, tx)
}

func FetchWithMajority[T any](ctx context.Context, nodes []*T, fetchFunc func(*T) (json.RawMessage, error)) (json.RawMessage, error) {
	type result struct {
		value json.RawMessage
		err   error
	}

	results := make(chan result, len(nodes))
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	for _, node := range nodes {
		go func(node *T) {
			value, err := fetchFunc(node)
			select {
			case results <- result{value, err}:
			case <-ctx.Done():
			}
		}(node)
	}

	var valueCount sync.Map
	majorityCount := (len(nodes) / 2) + 1
	responseCount := 0

	for {
		select {
		case res := <-results:
			if res.err == nil {
				hash, err := hashNormalizedJSON(res.value)
				if err != nil {
					return json.RawMessage{}, err
				}
				count, _ := valueCount.LoadOrStore(hash, 0)
				if countInt, ok := count.(int); ok {
					if countInt+1 >= majorityCount {
						cancel()
						return res.value, nil
					}
					valueCount.Store(hash, countInt+1)
				}
			}
			responseCount++
			if responseCount == len(nodes) {
				return json.RawMessage{}, errors.New("no majority consensus reached")
			}
		case <-ctx.Done():
			return json.RawMessage{}, ctx.Err()
		}
	}
}

func hashNormalizedJSON(data json.RawMessage) (string, error) {
	var obj interface{}
	if err := json.Unmarshal(data, &obj); err != nil {
		return "", err
	}

	switch v := obj.(type) {
	case map[string]interface{}:
		normalized, err := json.Marshal(normalizeJSON(v))
		if err != nil {
			return "", err
		}
		hash := sha256.Sum256(normalized)
		return hex.EncodeToString(hash[:]), nil
	case []interface{}:
		normalized, err := json.Marshal(normalizeJSONArray(v))
		if err != nil {
			return "", err
		}
		hash := sha256.Sum256(normalized)
		return hex.EncodeToString(hash[:]), nil
	default:
		return "", errors.New("unsupported JSON type")
	}
}

func normalizeJSON(obj map[string]interface{}) map[string]interface{} {
	for k, v := range obj {
		if m, ok := v.(map[string]interface{}); ok {
			obj[k] = normalizeJSON(m)
		}
	}

	// Sort keys to ensure consistent order
	keys := make([]string, 0, len(obj))
	for k := range obj {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	normalized := make(map[string]interface{}, len(obj))
	for _, k := range keys {
		normalized[k] = obj[k]
	}
	return normalized
}

func normalizeJSONArray(arr []interface{}) []interface{} {
	normalized := make([]interface{}, len(arr))
	for i, v := range arr {
		if m, ok := v.(map[string]interface{}); ok {
			normalized[i] = normalizeJSON(m)
		} else {
			normalized[i] = v
		}
	}
	return normalized
}
