package manifest

import "testing"

func TestCartfile(t *testing.T) {
	content := `github "Alamofire/Alamofire" ~> 5.9
git "https://github.com/ReactiveX/RxSwift.git" "6.8.0"
git "https://gitlab.com/acme/private.git"
binary "https://example.com/Framework.json"
`
	expect(t, "Cartfile", content, map[string][2]bool{"Alamofire/Alamofire": direct, "ReactiveX/RxSwift": direct}, "acme/private")
	expect(t, "Cartfile.private", `github "Quick/Nimble"`, map[string][2]bool{"Quick/Nimble": directDev})
	expect(t, "Cartfile.resolved", `github "Alamofire/Alamofire" "5.9.1"`, map[string][2]bool{"Alamofire/Alamofire": transitive})
}
