package manifest

import "testing"

func TestSbt(t *testing.T) {
	expect(t, "build.sbt", `ThisBuild / scalaVersion := "3.5.1"
libraryDependencies ++= Seq(
  "org.typelevel" %% "cats-core" % "2.12.0",
  "com.google.guava" % "guava" % "33.3.0-jre",
  "org.scalameta" %% "munit" % "1.0.2" % Test,
  "org.scala-js" %%% "scalajs-dom" % catsVersion % "test"
)
`, map[string][2]bool{
		"org.typelevel:cats-core_3": direct, "com.google.guava:guava": direct,
		"org.scalameta:munit_3": directDev, "org.scala-js:scalajs-dom_3": directDev,
	})
	expect(t, "project/plugins.sbt", `addSbtPlugin("org.scalameta" % "sbt-scalafmt" % "2.5.2")`,
		map[string][2]bool{"org.scalameta:sbt-scalafmt": directDev})
	expect(t, "build.sbt", `libraryDependencies += "com.lihaoyi" %% "os-lib" % "0.10.7"`,
		map[string][2]bool{"com.lihaoyi:os-lib_2.13": direct})
}

func TestDepsEDN(t *testing.T) {
	expect(t, "deps.edn", `{:paths ["src"]
 :deps {org.clojure/clojure {:mvn/version "1.12.0"}
        cheshire {:mvn/version "5.13.0"}
        io.github.clojure/tools.build {:git/tag "v0.10.5" :git/sha "2a21b7a"}
        lib/remote {:git/url "https://github.com/acme/remote.git" :git/sha "abc"}
        app/local {:local/root "../local"}}
 :aliases {:test {:extra-deps {lambdaisland/kaocha {:mvn/version "1.91.1392"}}}}}
`, map[string][2]bool{
		"org.clojure/clojure": direct, "cheshire/cheshire": direct, "clojure/tools.build": direct,
		"acme/remote": direct, "lambdaisland/kaocha": directDev,
	}, "app/local")
}

func TestProjectCLJ(t *testing.T) {
	expect(t, "project.clj", `(defproject app "0.1.0"
  :dependencies [[org.clojure/clojure "1.12.0"]
                 [ring "1.12.2" :exclusions [commons-io]]]
  :plugins [[lein-ancient "1.0.0-RC3"]]
  :profiles {:dev {:dependencies [[midje "1.10.9"]]}})
`, map[string][2]bool{
		"org.clojure/clojure": direct, "ring/ring": direct, "lein-ancient/lein-ancient": directDev, "midje/midje": directDev,
	}, "app/app", "commons-io/commons-io")
}
