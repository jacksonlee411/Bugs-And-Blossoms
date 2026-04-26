package cubebox

type ReadAPICatalogEntry struct {
	APIKey         string   `json:"api_key"`
	RequiredParams []string `json:"required_params"`
	OptionalParams []string `json:"optional_params"`
}

func (r *ExecutionRegistry) ReadAPICatalog() []ReadAPICatalogEntry {
	items := r.RegisteredExecutors()
	if len(items) == 0 {
		return nil
	}
	out := make([]ReadAPICatalogEntry, 0, len(items))
	for _, item := range items {
		out = append(out, ReadAPICatalogEntry{
			APIKey:         item.APIKey,
			RequiredParams: append([]string(nil), item.RequiredParams...),
			OptionalParams: append([]string(nil), item.OptionalParams...),
		})
	}
	return out
}
