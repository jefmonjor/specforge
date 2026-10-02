package domain

import (
	"errors"
	"strings"
	"testing"
)

func TestDiagnoseError(t *testing.T) {
	// 1. Knip
	knipErr := errors.New("Knip Violation: Dead code or unused dependencies detected.\nUnused files: src/legacy.ts")
	diagKnip := DiagnoseError(knipErr)
	if !strings.Contains(diagKnip.Title, "Knip") || diagKnip.ManualCmd != "npx knip" {
		t.Errorf("diagnóstico de Knip incorrecto: %+v", diagKnip)
	}

	// 2. DRY / jscpd
	dryErr := errors.New("DRY Violation (jscpd): Duplicated code found.\nClone found (ts)")
	diagDry := DiagnoseError(dryErr)
	if !strings.Contains(diagDry.Title, "jscpd") || !strings.Contains(diagDry.ManualCmd, "jscpd") {
		t.Errorf("diagnóstico de jscpd incorrecto: %+v", diagDry)
	}

	// 3. Stryker
	strykerErr := errors.New("Mutation Testing Failed: Surviving mutants detected. Tests are not robust enough.")
	diagStryker := DiagnoseError(strykerErr)
	if !strings.Contains(diagStryker.Title, "Stryker") || diagStryker.ManualCmd != "npx stryker run" {
		t.Errorf("diagnóstico de Stryker incorrecto: %+v", diagStryker)
	}

	// 4. YAGNI
	yagniErr := errors.New("❌ YAGNI Violation: El test pasa antes de escribir la implementación. Revisa el código o la prueba")
	diagYagni := DiagnoseError(yagniErr)
	if !strings.Contains(diagYagni.Title, "YAGNI") {
		t.Errorf("diagnóstico de YAGNI incorrecto: %+v", diagYagni)
	}

	// 5. Integridad Spec
	integErr := errors.New("la especificación ha sido modificada manualmente (esperado: abc, actual: def)")
	diagInteg := DiagnoseError(integErr)
	if !strings.Contains(diagInteg.Title, "Integridad Criptográfica") {
		t.Errorf("diagnóstico de integridad incorrecto: %+v", diagInteg)
	}

	// 6. Branch protection
	branchErr := errors.New("estás en la rama master. Te obligas a trabajar en una rama de feature")
	diagBranch := DiagnoseError(branchErr)
	if !strings.Contains(diagBranch.Title, "Protección de Ramas") {
		t.Errorf("diagnóstico de rama incorrecto: %+v", diagBranch)
	}

	// 7. Prueba visual de caja (no debe provocar pánicos)
	PrintDiagnosticBox(diagKnip)
}
