package legacy

import (
	"fmt"
	"slices"
	"strings"
)

// notes say what each technology means for a move to Java 21. They are
// facts about the platforms, not advice about this codebase: the agent and
// the developer decide the target.
var notes = map[string]map[string]string{
	"en": {
		"servlet":           "Jakarta EE 9+ renamed `javax.servlet` to `jakarta.servlet`; Spring Boot 3 and Tomcat 10+ require it.",
		"jsp":               "JSP still runs on Jakarta EE, but server-rendered views are usually rewritten (templates or a separate front end).",
		"struts1":           "Struts 1 reached end of life in 2013 and has unpatched vulnerabilities: its actions become controllers.",
		"struts2":           "Struts 2 is maintained but had critical CVEs; most migrations move to Spring MVC.",
		"ejb":               "EJB 2/3 session beans become plain services with dependency injection; `javax.ejb` is `jakarta.ejb` in Jakarta EE 9+.",
		"jpa":               "`javax.persistence` became `jakarta.persistence` (Hibernate 6, Spring Boot 3).",
		"hibernate":         "Hibernate 3/4 XML mappings and the Criteria API changed: target Hibernate 6 with `jakarta.persistence`.",
		"spring":            "Spring 6 / Spring Boot 3 need Java 17+ and the `jakarta.*` namespace; XML configuration can stay but is usually replaced.",
		"jdbc":              "Plain JDBC still works on Java 21; try-with-resources replaces manual close in `finally`.",
		"jaxrpc":            "JAX-RPC and Axis 1 are long dead and absent from Java 11+: SOAP endpoints need JAX-WS (Jakarta XML Web Services) or REST.",
		"jaxws":             "JAX-WS was removed from the JDK in Java 11: add Jakarta XML Web Services as a dependency (`jakarta.jws`).",
		"jaxb":              "JAXB was removed from the JDK in Java 11: add `jakarta.xml.bind` as a dependency.",
		"validation":        "`javax.validation` became `jakarta.validation` (Bean Validation 3).",
		"jms":               "`javax.jms` became `jakarta.jms` (Jakarta Messaging 3).",
		"log4j1":            "Log4j 1.x is end of life with known vulnerabilities: move to SLF4J with Logback or Log4j 2.",
		"commonslogging":    "Commons Logging can be bridged to SLF4J.",
		"junit3":            "JUnit 3 tests extend `TestCase`: JUnit 5 (Jupiter) uses annotations; the behaviour they check is still the reference.",
		"junit4":            "JUnit 4 runs on the JUnit 5 platform through the Vintage engine; new tests are written with Jupiter.",
		"legacycollections": "`Vector`, `Hashtable` and `Enumeration` are synchronized relics: `ArrayList`, `HashMap` and iterators replace them.",
		"legacydates":       "`java.time` (Java 8+) replaces `Date`, `Calendar` and `SimpleDateFormat`, which are mutable and not thread-safe.",
	},
	"es": {
		"servlet":           "Jakarta EE 9+ renombró `javax.servlet` a `jakarta.servlet`; Spring Boot 3 y Tomcat 10+ lo exigen.",
		"jsp":               "JSP sigue funcionando en Jakarta EE, pero las vistas en servidor suelen reescribirse (plantillas o un front aparte).",
		"struts1":           "Struts 1 está sin soporte desde 2013 y tiene vulnerabilidades sin parchear: sus acciones pasan a ser controladores.",
		"struts2":           "Struts 2 tiene mantenimiento pero sufrió CVE críticas; la mayoría de migraciones van a Spring MVC.",
		"ejb":               "Los session beans EJB 2/3 pasan a ser servicios con inyección de dependencias; `javax.ejb` es `jakarta.ejb` en Jakarta EE 9+.",
		"jpa":               "`javax.persistence` pasó a `jakarta.persistence` (Hibernate 6, Spring Boot 3).",
		"hibernate":         "Los mapeos XML y la API Criteria de Hibernate 3/4 cambiaron: el destino es Hibernate 6 con `jakarta.persistence`.",
		"spring":            "Spring 6 / Spring Boot 3 necesitan Java 17+ y el espacio `jakarta.*`; la configuración XML puede quedarse, pero suele sustituirse.",
		"jdbc":              "JDBC directo sigue funcionando en Java 21; try-with-resources sustituye al cierre manual en `finally`.",
		"jaxrpc":            "JAX-RPC y Axis 1 están muertos y no existen en Java 11+: los endpoints SOAP necesitan JAX-WS (Jakarta XML Web Services) o REST.",
		"jaxws":             "JAX-WS salió del JDK en Java 11: se añade Jakarta XML Web Services como dependencia (`jakarta.jws`).",
		"jaxb":              "JAXB salió del JDK en Java 11: se añade `jakarta.xml.bind` como dependencia.",
		"validation":        "`javax.validation` pasó a `jakarta.validation` (Bean Validation 3).",
		"jms":               "`javax.jms` pasó a `jakarta.jms` (Jakarta Messaging 3).",
		"log4j1":            "Log4j 1.x está sin soporte y con vulnerabilidades conocidas: SLF4J con Logback o Log4j 2.",
		"commonslogging":    "Commons Logging se puede puentear a SLF4J.",
		"junit3":            "Los tests JUnit 3 heredan de `TestCase`: JUnit 5 (Jupiter) usa anotaciones; el comportamiento que comprueban sigue siendo la referencia.",
		"junit4":            "JUnit 4 corre en la plataforma JUnit 5 con el motor Vintage; los tests nuevos se escriben con Jupiter.",
		"legacycollections": "`Vector`, `Hashtable` y `Enumeration` son reliquias sincronizadas: los sustituyen `ArrayList`, `HashMap` e iteradores.",
		"legacydates":       "`java.time` (Java 8+) sustituye a `Date`, `Calendar` y `SimpleDateFormat`, que son mutables y no son thread-safe.",
	},
}

type reportLabels struct {
	title, build, release, unknown, size, sources, tests, lines, techs, none, packages, notesTitle, truncated string
}

var reportCatalog = map[string]reportLabels{
	"en": {"Legacy inventory", "Build", "Declared Java release", "not declared", "Size", "source files", "test files", "lines",
		"Technologies found (from imports and files)", "none recognised", "Largest packages", "What each one means for Java 21",
		"The scan stopped at 50,000 files: the counts are partial."},
	"es": {"Inventario legacy", "Build", "Release de Java declarada", "no declarada", "Tamaño", "ficheros fuente", "ficheros de test", "líneas",
		"Tecnologías encontradas (por imports y ficheros)", "ninguna reconocida", "Paquetes más grandes", "Qué supone cada una para Java 21",
		"El escaneo se detuvo en 50.000 ficheros: las cifras son parciales."},
}

// Markdown renders the inventory for people and for the agent.
func (inv Inventory) Markdown(lang, source string) string {
	l, ok := reportCatalog[lang]
	if !ok {
		l, lang = reportCatalog["en"], "en"
	}
	var b strings.Builder
	fmt.Fprintf(&b, "# %s · `%s`\n\n", l.title, source)
	if inv.Truncated {
		b.WriteString("> " + l.truncated + "\n\n")
	}
	release := inv.JavaRelease
	if release == "" {
		release = l.unknown
	}
	fmt.Fprintf(&b, "- **%s**: %s\n- **%s**: %s\n", l.build, inv.Build, l.release, release)
	fmt.Fprintf(&b, "- **%s**: %d %s · %d %s · %d %s\n", l.size, inv.SourceFiles, l.sources, inv.TestFiles, l.tests, inv.Lines, l.lines)

	exts := make([]string, 0, len(inv.Files))
	for e := range inv.Files {
		exts = append(exts, e)
	}
	slices.SortFunc(exts, func(a, b string) int {
		if inv.Files[a] != inv.Files[b] {
			return inv.Files[b] - inv.Files[a]
		}
		return strings.Compare(a, b)
	})
	var parts []string
	for i, e := range exts {
		if i == 10 {
			break
		}
		name := e
		if name == "" {
			name = "(none)"
		}
		parts = append(parts, fmt.Sprintf("%s %d", name, inv.Files[e]))
	}
	if len(parts) > 0 {
		b.WriteString("- " + strings.Join(parts, " · ") + "\n")
	}

	fmt.Fprintf(&b, "\n## %s\n\n", l.techs)
	if len(inv.Frameworks) == 0 {
		b.WriteString("- " + l.none + "\n")
	}
	for _, f := range inv.Frameworks {
		fmt.Fprintf(&b, "- **%s**: %d · `%s`\n", f.Name, f.Files, f.Example)
	}
	if len(inv.Packages) > 0 {
		fmt.Fprintf(&b, "\n## %s\n\n", l.packages)
		for i, p := range inv.Packages {
			if i == 15 {
				break
			}
			fmt.Fprintf(&b, "- `%s`: %d\n", p.Name, p.Files)
		}
	}
	var lines []string
	for _, f := range inv.Frameworks {
		if n := notes[lang][f.ID]; n != "" {
			lines = append(lines, fmt.Sprintf("- **%s**: %s", f.Name, n))
		}
	}
	if release := ReleaseNumber(inv.JavaRelease); release > 0 && release < 21 {
		lines = append([]string{fmt.Sprintf("- **Java %s → 21**: %s", inv.JavaRelease, releaseNote(lang, release))}, lines...)
	}
	if len(lines) > 0 {
		fmt.Fprintf(&b, "\n## %s\n\n%s\n", l.notesTitle, strings.Join(lines, "\n"))
	}
	return b.String()
}

func releaseNote(lang string, from int) string {
	var en, es []string
	if from < 8 {
		en = append(en, "lambdas, streams, `Optional` and `java.time` arrive with Java 8")
		es = append(es, "lambdas, streams, `Optional` y `java.time` llegan con Java 8")
	}
	if from < 11 {
		en = append(en, "Java 11 removed Java EE and CORBA from the JDK (JAXB, JAX-WS, `javax.annotation`)")
		es = append(es, "Java 11 sacó Java EE y CORBA del JDK (JAXB, JAX-WS, `javax.annotation`)")
	}
	if from < 17 {
		en = append(en, "Java 17 strongly encapsulates JDK internals (`sun.*` access fails)")
		es = append(es, "Java 17 encapsula los internos del JDK (el acceso a `sun.*` falla)")
	}
	en = append(en, "records, `var`, text blocks, switch expressions and virtual threads are available")
	es = append(es, "records, `var`, bloques de texto, switch expressions e hilos virtuales están disponibles")
	if lang == "es" {
		return strings.Join(es, "; ") + "."
	}
	return strings.Join(en, "; ") + "."
}
