package queue

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWebhookPayload_Serialization(t *testing.T) {
	payload := WebhookPayload{
		CallbackID: 123,
		URL:        "https://example.com/callback",
		Body:       `{"order_sn":"ORD-001","status":"paid"}`,
	}

	data, err := json.Marshal(payload)
	require.NoError(t, err)

	var decoded WebhookPayload
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, payload, decoded)
}

func TestDockingPayload_Serialization(t *testing.T) {
	payload := DockingPayload{
		TaskID:  456,
		OrderSN: "ORD-2024-001",
	}

	data, err := json.Marshal(payload)
	require.NoError(t, err)

	var decoded DockingPayload
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, payload, decoded)
}

func TestReconciliationPayload_Serialization(t *testing.T) {
	payload := ReconciliationPayload{
		Type: "balance_check",
	}

	data, err := json.Marshal(payload)
	require.NoError(t, err)

	var decoded ReconciliationPayload
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, payload, decoded)
}

func TestAnalyticsPayload_Serialization(t *testing.T) {
	payload := AnalyticsPayload{
		Date: "2026-07-04",
	}

	data, err := json.Marshal(payload)
	require.NoError(t, err)

	var decoded AnalyticsPayload
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, payload, decoded)
}

func TestCardImportPayload_Serialization(t *testing.T) {
	payload := CardImportPayload{
		GoodsID:   789,
		BatchName: "batch-001",
		Contents:  []string{"card1", "card2", "card3"},
	}

	data, err := json.Marshal(payload)
	require.NoError(t, err)

	var decoded CardImportPayload
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, payload, decoded)
}

func TestCardImportPayload_EmptyContents(t *testing.T) {
	payload := CardImportPayload{
		GoodsID:   1,
		BatchName: "empty-batch",
		Contents:  []string{},
	}

	data, err := json.Marshal(payload)
	require.NoError(t, err)

	var decoded CardImportPayload
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, uint(1), decoded.GoodsID)
	assert.Equal(t, "empty-batch", decoded.BatchName)
	assert.Empty(t, decoded.Contents)
}

func TestTaskTypes_Constants(t *testing.T) {
	assert.Equal(t, "webhook:deliver", TypeWebhookDeliver)
	assert.Equal(t, "docking:submit", TypeDockingSubmit)
	assert.Equal(t, "reconciliation:run", TypeReconciliationRun)
	assert.Equal(t, "analytics:aggregate", TypeAnalyticsAggregate)
	assert.Equal(t, "card:import", TypeCardImport)
}

func TestQueueClient_Disabled(t *testing.T) {
	// 空地址创建禁用客户端
	client := NewQueueClient("", "", 0)
	assert.False(t, client.IsEnabled())

	// 禁用客户端Enqueue应返回error
	err := client.Enqueue(context.Background(), TypeWebhookDeliver, WebhookPayload{})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not enabled")
}

func TestQueueClient_Close_NilClient(t *testing.T) {
	client := NewQueueClient("", "", 0)
	err := client.Close()
	assert.NoError(t, err)
}

func TestPayload_JsonFieldNames(t *testing.T) {
	// Verify JSON field names are correct
	data, _ := json.Marshal(WebhookPayload{CallbackID: 1, URL: "http://x", Body: "b"})
	assert.Contains(t, string(data), `"callback_id"`)
	assert.Contains(t, string(data), `"url"`)
	assert.Contains(t, string(data), `"body"`)

	data, _ = json.Marshal(DockingPayload{TaskID: 1, OrderSN: "sn"})
	assert.Contains(t, string(data), `"task_id"`)
	assert.Contains(t, string(data), `"order_sn"`)

	data, _ = json.Marshal(CardImportPayload{GoodsID: 1, BatchName: "b", Contents: []string{"c"}})
	assert.Contains(t, string(data), `"goods_id"`)
	assert.Contains(t, string(data), `"batch_name"`)
	assert.Contains(t, string(data), `"contents"`)
}
