package aiorders

import "testing"

func TestRussianExplanatoryText(t *testing.T) {
	if got := DisplayText("Тип single/Double, покрытие PVC."); got != "Тип Одностворчатая/Двустворчатая, покрытие ПВХ." {
		t.Fatal(got)
	}
	if got := DisplayText("SingleBrand"); got != "SingleBrand" {
		t.Fatal(got)
	}
}
