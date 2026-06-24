package container

import "fmt"

// allowedPairs is the build/run engine compatibility matrix. It must stay in
// lockstep with the runtime_pairs section of the embedded recipe registry
// (static/files/recipes/recipes.yaml); a cross-check test in
// internal/recipes enforces the correspondence.
var allowedPairs = map[[2]Engine]bool{
	{EngineDocker, EngineDocker}:  true,
	{EnginePodman, EnginePodman}:  true,
	{EngineNerdctl, EngineDocker}: true,
	{EngineNerdctl, EnginePodman}: true,
	{EngineDocker, EngineK3s}:     true,
	{EnginePodman, EngineK3s}:     true,
}

// ValidatePair reports whether the build/run engine combination is allowed.
func ValidatePair(build, run Engine) error {
	if allowedPairs[[2]Engine{build, run}] {
		return nil
	}
	return wrap(ErrUnsupportedCapability,
		fmt.Errorf("unsupported build/run engine pair %s/%s", build, run),
		"see the Agent Environment reference for the allowed runtime pairs")
}
