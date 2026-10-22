package manifest

import "testing"

func TestVcpkgJSON(t *testing.T) {
	expect(t, "vcpkg.json", `{
  "name": "app",
  "dependencies": ["fmt", {"name": "boost-asio"}, {"name": "vcpkg-cmake", "host": true}]
}`, map[string][2]bool{"fmt": direct, "boost-asio": direct, "vcpkg-cmake": directDev}, "app")
}

func TestConan(t *testing.T) {
	expect(t, "conanfile.txt", `[requires]
zlib/1.3.1
openssl/3.3.2@

[tool_requires]
cmake/3.30.1

[generators]
CMakeDeps
`, map[string][2]bool{"zlib": direct, "openssl": direct, "cmake": directDev}, "CMakeDeps")
	expect(t, "conanfile.py", `class App(ConanFile):
    requires = "fmt/11.0.2", "spdlog/1.14.1"
    tool_requires = "ninja/1.12.1"

    def requirements(self):
        self.requires("boost/1.86.0")
        self.test_requires("gtest/1.15.0")
`, map[string][2]bool{"fmt": direct, "spdlog": direct, "ninja": directDev, "boost": direct, "gtest": directDev})
	expect(t, "conan.lock", `{"version": "0.5", "requires": ["zlib/1.3.1#abc%1700000000.0"], "build_requires": ["cmake/3.30.1#def"]}`,
		map[string][2]bool{"zlib": transitive, "cmake": {false, true}})
}
