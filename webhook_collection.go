package webhook_core

var webhooks []Webhook

// RegisterWebhook register a new webhook
func RegisterWebhook(webhook Webhook) {
	webhooks = append(webhooks, webhook)
}

// GetRegisteredWebhooks get list of all registered webhooks
func GetRegisteredWebhooks() []Webhook {
	return webhooks
}
