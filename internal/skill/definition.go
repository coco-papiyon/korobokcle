package skill

type Definition struct {
	Name       string `yaml:"name"`
	Provider   string `yaml:"provider"`
	PromptFile string `yaml:"-"`
}
