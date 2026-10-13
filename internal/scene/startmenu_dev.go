//go:build !demo

package scene

func init() {
	devMenuButtons = func(s *StartMenu) []btnDef {
		return []btnDef{
			{"Scenario Editor", func() {
				s.app.PushScene(NewEditorScenarioPicker())
			}},
			{"Testbeds", func() {
				s.app.PushScene(NewTestbedMenu())
			}},
		}
	}
}
