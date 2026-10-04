package legacy

import (
	"strings"
	"testing"
)

func TestCitationsAndVerify(t *testing.T) {
	doc := "Rule from `src/main/java/com/acme/payroll/PayrollServlet.java:3` and `src/main/java/com/acme/payroll/dao/EmployeeDao.java:2-3`.\n" +
		"Again `src/main/java/com/acme/payroll/PayrollServlet.java:3`; invented `src/Missing.java:1`; too far `pom.xml:900`; no line `src/main/webapp/payroll.jsp`; not a path `INV-01`, `java.util.Vector`, `1.6`."
	cs := Citations(doc)
	if len(cs) != 5 || cs[1].From != 2 || cs[1].To != 3 || cs[4].From != 0 {
		t.Fatalf("citations %+v", cs)
	}
	problems := Verify(legacyRepo, cs)
	if len(problems) != 2 || !strings.Contains(problems[0], "src/Missing.java:1") || !strings.Contains(problems[1], "pom.xml:900") {
		t.Fatalf("problems %v", problems)
	}
}
