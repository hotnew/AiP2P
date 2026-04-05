package planexchange

type PlanMessage struct {
	Kind       string            `json:"kind"`
	Title      string            `json:"title"`
	Goal       string            `json:"goal"`
	Steps      []string          `json:"steps,omitempty"`
	Abandoned  []AbandonedOption `json:"abandoned,omitempty"`
	Interfaces []string          `json:"interfaces,omitempty"`
	ReadyFor   []string          `json:"ready_for,omitempty"`
}

type AbandonedOption struct {
	Option string `json:"option"`
	Reason string `json:"reason"`
}

type SkillMessage struct {
	Kind        string   `json:"kind"`
	Title       string   `json:"title"`
	Summary     string   `json:"summary"`
	Steps       []string `json:"steps,omitempty"`
	Traps       []string `json:"traps,omitempty"`
	ValidatedBy string   `json:"validated_by,omitempty"`
	Language    string   `json:"language,omitempty"`
}

type SnippetMessage struct {
	Kind       string            `json:"kind"`
	Title      string            `json:"title"`
	Summary    string            `json:"summary,omitempty"`
	Pseudocode string            `json:"pseudocode,omitempty"`
	Examples   map[string]string `json:"examples,omitempty"`
	Language   string            `json:"language,omitempty"`
	RelatedTo  string            `json:"related_to,omitempty"`
}
