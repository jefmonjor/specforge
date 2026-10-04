package [[.Package]];

import static com.tngtech.archunit.lang.syntax.ArchRuleDefinition.noClasses;

import com.tngtech.archunit.core.importer.ImportOption;
import com.tngtech.archunit.junit.AnalyzeClasses;
import com.tngtech.archunit.junit.ArchTest;
import com.tngtech.archunit.lang.ArchRule;

/** The layers of a hexagonal architecture, checked on every test run. */
@AnalyzeClasses(packages = "[[.Package]]", importOptions = ImportOption.DoNotIncludeTests.class)
class ArchitectureTest {

    @ArchTest
    static final ArchRule domainDependsOnNothingOutside = noClasses()
            .that().resideInAPackage("..domain..")
            .should().dependOnClassesThat()
            .resideInAnyPackage("..application..", "..adapters..", "jakarta..", "javax..", "org.springframework..")
            .allowEmptyShould(true);

    @ArchTest
    static final ArchRule applicationDoesNotKnowTheAdapters = noClasses()
            .that().resideInAPackage("..application..")
            .should().dependOnClassesThat().resideInAPackage("..adapters..")
            .allowEmptyShould(true);

    @ArchTest
    static final ArchRule noJavaEeLeftovers = noClasses()
            .should().dependOnClassesThat()
            .resideInAnyPackage("javax.servlet..", "javax.persistence..", "javax.validation..", "javax.ejb..",
                    "org.apache.log4j..", "junit.framework..")
            .allowEmptyShould(true);

    @ArchTest
    static final ArchRule noLegacyCollections = noClasses()
            .should().dependOnClassesThat()
            .haveFullyQualifiedName("java.util.Vector")
            .orShould().dependOnClassesThat().haveFullyQualifiedName("java.util.Hashtable")
            .allowEmptyShould(true);
}
