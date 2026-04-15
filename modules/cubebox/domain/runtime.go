package domain

type RuntimeComponentStatus struct {
	Healthy string `json:"healthy"`
	Reason  string `json:"reason,omitempty"`
}

type RuntimeCapabilities struct {
	ConversationEnabled bool `json:"conversation_enabled"`
	FilesEnabled        bool `json:"files_enabled"`
	AgentsUIEnabled     bool `json:"agents_ui_enabled"`
	AgentsWriteEnabled  bool `json:"agents_write_enabled"`
	MemoryEnabled       bool `json:"memory_enabled"`
	WebSearchEnabled    bool `json:"web_search_enabled"`
	FileSearchEnabled   bool `json:"file_search_enabled"`
	MCPEnabled          bool `json:"mcp_enabled"`
}

type RuntimeStatus struct {
	Status              string                 `json:"status"`
	CheckedAt           string                 `json:"checked_at"`
	Frontend            RuntimeComponentStatus `json:"frontend"`
	Backend             RuntimeComponentStatus `json:"backend"`
	KnowledgeRuntime    RuntimeComponentStatus `json:"knowledge_runtime"`
	ModelGateway        RuntimeComponentStatus `json:"model_gateway"`
	FileStore           RuntimeComponentStatus `json:"file_store"`
	RetiredCapabilities []string               `json:"retired_capabilities"`
	Capabilities        RuntimeCapabilities    `json:"capabilities"`
}
