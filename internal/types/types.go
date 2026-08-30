package types

type (
	Provider     string
	Architecture string
	Stack        string
)

type Spec struct {
	Provider      Provider       `yaml:"provider,omitempty"`
	Architectures []Architecture `yaml:"architectures,omitempty"`
	Inputs        []string       `yaml:"inputs,omitempty"`
}

type Manifest struct {
	Inputs        []string              `yaml:"inputs"`
	Providers     map[Provider]Spec     `yaml:"providers"`
	Architectures map[Architecture]Spec `yaml:"architectures"`
	Stacks        map[Stack]Spec        `yaml:"stacks"`
}
