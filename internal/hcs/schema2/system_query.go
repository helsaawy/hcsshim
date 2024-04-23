package hcsschema

type SystemQuery struct {
	IDs    []string     `json:"Ids,omitempty"`
	Names  []string     `json:",omitempty"`
	Types  []SystemType `json:",omitempty"`
	Owners []string     `json:",omitempty"`
}
