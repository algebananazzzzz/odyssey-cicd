package types

type (
	Provider     string
	Architecture string
	Stack        string
)

type Input struct {
	Optional bool `yaml:"optional"`
}

type Github struct {
	Variables []string `yaml:"variables"`
	Secrets   []string `yaml:"secrets"`
}

type Spec struct {
	Provider      Provider         `yaml:"provider,omitempty"`
	Architectures []Architecture   `yaml:"architectures,omitempty"`
	Inputs        map[string]Input `yaml:"inputs,omitempty"`
	Github        Github           `yaml:"github,omitempty"`
}

type Manifest struct {
	Inputs        map[string]Input      `yaml:"inputs"`
	Providers     map[Provider]Spec     `yaml:"providers"`
	Architectures map[Architecture]Spec `yaml:"architectures"`
	Stacks        map[Stack]Spec        `yaml:"stacks"`
}
