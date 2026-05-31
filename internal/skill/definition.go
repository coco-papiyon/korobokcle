package skill

type Definition struct {
	Name            string   `yaml:"name" json:"name"`
	Title           string   `yaml:"title" json:"title"`
	Role            string   `yaml:"role" json:"role"`
	PromptTemplates []string `yaml:"promptTemplates" json:"promptTemplates"`
	PromptFile      string   `yaml:"-" json:"-"`
}
