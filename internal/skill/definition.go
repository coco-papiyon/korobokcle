package skill

type Definition struct {
	Name       string        `yaml:"name"`
	Provider   string        `yaml:"provider"`
	Inputs     []string      `yaml:"inputs"`
	Outputs    []string      `yaml:"outputs"`
	Artifacts  ArtifactBlock `yaml:"artifacts"`
	PromptFile string        `yaml:"-"`
}

type ArtifactBlock struct {
	OutputFile string `yaml:"output_file"`
}
