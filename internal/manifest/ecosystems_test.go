package manifest

import "testing"

func TestGradle(t *testing.T) {
	expect(t, "app/build.gradle.kts", `plugins {
    id("com.android.application")
}
dependencies {
    implementation("com.squareup.okhttp3:okhttp:4.12.0")
    implementation(platform("androidx.compose:compose-bom:2024.10.00"))
    api("io.reactivex.rxjava3:rxjava:3.1.8")
    kapt("com.google.dagger:hilt-compiler:2.51")
    testImplementation("junit:junit:4.13.2")
    androidTestImplementation("androidx.test.espresso:espresso-core:3.5.1")
    implementation(libs.androidx.core.ktx)
    implementation(project(":core"))
}
`, map[string][2]bool{
		"com.squareup.okhttp3:okhttp": direct, "androidx.compose:compose-bom": direct, "io.reactivex.rxjava3:rxjava": direct,
		"com.google.dagger:hilt-compiler": direct, "junit:junit": directDev, "androidx.test.espresso:espresso-core": directDev,
	}, "com.android.application")
	expect(t, "build.gradle", `dependencies {
    implementation 'com.google.code.gson:gson:2.11.0'
    testImplementation group: 'org.mockito', name: 'mockito-core', version: '5.0.0'
    classpath "com.android.tools.build:gradle:8.5.0"
}
repositories { maven { url "https://jitpack.io" } }
`, map[string][2]bool{"com.google.code.gson:gson": direct, "org.mockito:mockito-core": directDev, "com.android.tools.build:gradle": direct})
}

func TestVersionCatalog(t *testing.T) {
	expect(t, "gradle/libs.versions.toml", `[versions]
okhttp = "4.12.0"

[libraries]
okhttp = { module = "com.squareup.okhttp3:okhttp", version.ref = "okhttp" }
androidx-core-ktx = { group = "androidx.core", name = "core-ktx", version = "1.13.1" }
timber = "com.jakewharton.timber:timber:5.0.1"

[plugins]
android-application = { id = "com.android.application", version = "8.5.0" }
`, map[string][2]bool{"com.squareup.okhttp3:okhttp": direct, "androidx.core:core-ktx": direct, "com.jakewharton.timber:timber": direct}, "com.android.application", "okhttp")
}

func TestPomXML(t *testing.T) {
	expect(t, "pom.xml", `<project>
  <groupId>com.acme</groupId>
  <artifactId>app</artifactId>
  <dependencies>
    <dependency>
      <groupId>com.fasterxml.jackson.core</groupId>
      <artifactId>jackson-databind</artifactId>
      <version>2.17.0</version>
    </dependency>
    <dependency>
      <groupId>org.junit.jupiter</groupId>
      <artifactId>junit-jupiter</artifactId>
      <scope>test</scope>
    </dependency>
    <dependency>
      <groupId>${project.groupId}</groupId>
      <artifactId>core</artifactId>
    </dependency>
  </dependencies>
</project>`, map[string][2]bool{"com.fasterxml.jackson.core:jackson-databind": direct, "org.junit.jupiter:junit-jupiter": directDev}, "com.acme:app")
}

func TestGemfile(t *testing.T) {
	expect(t, "Gemfile", `source "https://rubygems.org"
gem "rails", "~> 7.1"
gem 'pg'

group :development, :test do
  gem "rspec-rails"
  if ENV["CI"]
    gem "simplecov"
  end
end

gem "puma"
`, map[string][2]bool{"rails": direct, "pg": direct, "rspec-rails": directDev, "simplecov": directDev, "puma": direct})
	expect(t, "Gemfile.lock", `GEM
  remote: https://rubygems.org/
  specs:
    actionpack (7.1.3)
      rack (>= 2.2.4)
    rack (3.0.9)

PLATFORMS
  ruby

DEPENDENCIES
  rails (~> 7.1)
`, map[string][2]bool{"actionpack": transitive, "rack": transitive}, "rails", "ruby")
}

func TestComposer(t *testing.T) {
	expect(t, "composer.json", `{"require": {"php": ">=8.2", "ext-json": "*", "Guzzlehttp/Guzzle": "^7"}, "require-dev": {"phpunit/phpunit": "^11"}}`,
		map[string][2]bool{"guzzlehttp/guzzle": direct, "phpunit/phpunit": directDev}, "php", "ext-json")
	expect(t, "composer.lock", `{"packages": [{"name": "psr/http-message"}], "packages-dev": [{"name": "sebastian/diff"}]}`,
		map[string][2]bool{"psr/http-message": transitive, "sebastian/diff": {false, true}})
}

func TestPubspec(t *testing.T) {
	expect(t, "pubspec.yaml", `name: app
environment:
  sdk: ">=3.0.0 <4.0.0"

dependencies:
  flutter:
    sdk: flutter
  provider: ^6.1.2
  http: ^1.2.0
  local_pkg:
    path: ../local_pkg

dev_dependencies:
  flutter_test:
    sdk: flutter
  mocktail: ^1.0.0
`, map[string][2]bool{"provider": direct, "http": direct, "mocktail": directDev}, "flutter", "flutter_test", "local_pkg", "sdk", "name")
	expect(t, "pubspec.lock", `packages:
  provider:
    dependency: "direct main"
    description:
      name: provider
      url: "https://pub.dev"
    source: hosted
    version: "6.1.2"
  nested:
    dependency: transitive
    description:
      name: nested
    source: hosted
    version: "1.0.0"
  flutter:
    dependency: "direct main"
    description: flutter
    source: sdk
    version: "0.0.0"
sdks:
  dart: ">=3.0.0 <4.0.0"
`, map[string][2]bool{"provider": direct, "nested": transitive}, "flutter", "dart")
}

func TestSwiftAndCocoaPods(t *testing.T) {
	expect(t, "Package.swift", `let package = Package(
    dependencies: [
        .package(url: "https://github.com/apple/swift-argument-parser", from: "1.3.0"),
        .package(name: "Alamofire", url: "https://github.com/Alamofire/Alamofire.git", .upToNextMajor(from: "5.9.0")),
        .package(path: "../Local"),
    ]
)`, map[string][2]bool{"https://github.com/apple/swift-argument-parser": direct, "https://github.com/Alamofire/Alamofire.git": direct})
	expect(t, "Package.resolved", `{"pins": [{"identity": "swift-log", "kind": "remoteSourceControl", "location": "https://github.com/apple/swift-log.git"}], "version": 2}`,
		map[string][2]bool{"https://github.com/apple/swift-log.git": transitive})
	expect(t, "Package.resolved", `{"object": {"pins": [{"package": "Nuke", "repositoryURL": "https://github.com/kean/Nuke"}]}, "version": 1}`,
		map[string][2]bool{"https://github.com/kean/Nuke": transitive})
	expect(t, "Podfile.lock", `PODS:
  - Alamofire (5.9.1)
  - Firebase/Core (10.0.0):
    - FirebaseAnalytics (= 10.0.0)
  - FirebaseAnalytics (10.0.0)

DEPENDENCIES:
  - Alamofire (~> 5.9)
  - Firebase/Core

SPEC REPOS:
  trunk:
    - Alamofire
`, map[string][2]bool{"Alamofire": direct, "Firebase": direct, "FirebaseAnalytics": transitive}, "trunk")
}

func TestDotnet(t *testing.T) {
	expect(t, "src/App/App.csproj", `<Project Sdk="Microsoft.NET.Sdk">
  <ItemGroup>
    <PackageReference Include="Newtonsoft.Json" Version="13.0.3" />
    <PackageReference Include="Serilog">
      <Version>3.1.1</Version>
    </PackageReference>
    <ProjectReference Include="..\Core\Core.csproj" />
  </ItemGroup>
</Project>`, map[string][2]bool{"Newtonsoft.Json": direct, "Serilog": direct}, `..\Core\Core.csproj`)
	expect(t, "packages.lock.json", `{"version": 1, "dependencies": {"net8.0": {
		"Serilog": {"type": "Direct", "resolved": "3.1.1"},
		"Microsoft.Extensions.Logging": {"type": "Transitive"},
		"Core": {"type": "Project"}
	}}}`, map[string][2]bool{"Serilog": direct, "Microsoft.Extensions.Logging": transitive}, "Core")
}
