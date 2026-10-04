# Patrones de Referencia (Golden Masters)

## 1. Value Object Inmutable (Java)
```java
public record Iban(String value) {
    public Iban {
        if (value == null || !value.matches("^[A-Z]{2}[0-9]{2}[A-Z0-9]{1,30}$")) {
            throw new IllegalArgumentException("Formato de IBAN inválido: " + value);
        }
    }
}
```

## 2. Caso de Uso Hexagonal (Go)
```go
type TransferFundsUseCase struct {
    accountRepo ports.AccountRepository
    notifier    ports.NotificationService
}

func (uc *TransferFundsUseCase) Execute(ctx context.Context, cmd domain.TransferCommand) error {
    origin, err := uc.accountRepo.FindByID(ctx, cmd.OriginID)
    if err != nil {
        return err
    }
    if err := origin.Debit(cmd.Amount); err != nil {
        return err
    }
    return uc.accountRepo.Save(ctx, origin)
}
```

## 3. Configuración de Sonda de Salud (Spring Boot application.yaml)
```yaml
management:
  endpoints:
    web:
      base-path: /
      exposure:
        include: health, info
  endpoint:
    health:
      show-details: never
```
