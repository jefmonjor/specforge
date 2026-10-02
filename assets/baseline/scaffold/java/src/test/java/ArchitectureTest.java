package {{PACKAGE_NAME}};

import com.tngtech.archunit.core.importer.ImportOption;
import com.tngtech.archunit.junit.AnalyzeClasses;
import com.tngtech.archunit.junit.ArchTest;
import com.tngtech.archunit.lang.ArchRule;

import static com.tngtech.archunit.lang.syntax.ArchRuleDefinition.classes;
import static com.tngtech.archunit.lang.syntax.ArchRuleDefinition.noClasses;

/**
 * SDDFramework — Gate de Calidad y Clean Architecture (Software Craftsmanship).
 *
 * Esta suite se ejecuta en 'mvn test' y garantiza de forma autoritativa e inmutable
 * que ni el Agente de IA ni ningún desarrollador rompan los principios SOLID y la
 * separación de capas de la Arquitectura Hexagonal.
 */
@AnalyzeClasses(packages = "{{PACKAGE_NAME}}", importOptions = ImportOption.DoNotIncludeTests.class)
public class ArchitectureTest {

    /**
     * Regla de Oro 1: La capa de DOMINIO es pura.
     * Cero dependencias de Spring, Jakarta Persistence (JPA), o librerías de infraestructura.
     */
    @ArchTest
    public static final ArchRule domain_must_be_free_of_framework_dependencies =
            noClasses()
                    .that().resideInAPackage("..domain..")
                    .should().dependOnClassesThat()
                    .resideInAnyPackage(
                            "..springframework..",
                            "..jakarta.persistence..",
                            "..javax.persistence..",
                            "..hibernate.."
                    )
                    .because("El dominio debe ser puro y no depender de frameworks ni detalles de persistencia (Clean Architecture).");

    /**
     * Regla de Oro 2: Los adaptadores no deben conocerse entre sí.
     * Un controlador REST (in) no puede depender directamente de un repositorio JPA (out).
     */
    @ArchTest
    public static final ArchRule adapters_must_not_depend_on_other_adapters =
            noClasses()
                    .that().resideInAPackage("..infrastructure.adapters.in..")
                    .should().dependOnClassesThat()
                    .resideInAPackage("..infrastructure.adapters.out..")
                    .because("La comunicación entre adaptadores debe canalizarse estrictamente a través de los casos de uso (DIP).");

    /**
     * Regla de Oro 3: Los Puertos deben ser Interfaces.
     * Garantiza el principio de Inversión de Dependencias (DIP) y Segregación de Interfaces (ISP).
     */
    @ArchTest
    public static final ArchRule ports_must_be_interfaces =
            classes()
                    .that().resideInAPackage("..ports..")
                    .should().beInterfaces()
                    .because("Los puertos (entrada y salida) deben ser abstracciones puras / contratos (DIP e ISP).");
}
