package manifest

import "testing"

func TestSetupPy(t *testing.T) {
	expect(t, "setup.py", `from setuptools import setup
setup(
    name="app",
    install_requires=["requests>=2", 'click'],
    tests_require=["pytest"],
    extras_require={"yaml": ["PyYAML>=6"]},
)
`, map[string][2]bool{"requests": direct, "click": direct, "pytest": directDev, "pyyaml": direct}, "app")
}

func TestSetupCfg(t *testing.T) {
	expect(t, "setup.cfg", `[metadata]
name = app

[options]
install_requires =
    attrs>=23
    importlib-metadata; python_version<"3.10"

[options.extras_require]
test =
    pytest
toml = tomli
`, map[string][2]bool{"attrs": direct, "importlib-metadata": direct, "pytest": directDev, "tomli": direct}, "app")
}

func TestCondaEnvironment(t *testing.T) {
	expect(t, "environment.yml", `name: science
channels:
  - conda-forge
dependencies:
  - python=3.12
  - numpy>=1.26
  - conda-forge::pandas
  - pip
  - pip:
    - rich==13.7
`, map[string][2]bool{"numpy": direct, "pandas": direct, "rich": direct}, "python", "pip", "conda-forge", "science")
	dependencies := names(Parse("environment.yml", []byte("dependencies:\n  - numpy\n  - pip:\n    - rich\n")))
	if dependencies["numpy"].Ecosystem != Conda || dependencies["rich"].Ecosystem != PyPI {
		t.Errorf("conda ecosystems = %+v", dependencies)
	}
}
