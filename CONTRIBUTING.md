# Contribuir a IroncladDTL

## Flujo

1. Crea una rama desde `main`.
2. Mantén cada cambio dentro de un dominio claro.
3. Añade pruebas de camino nominal y rechazo.
4. Actualiza el contrato público si cambia una entrada o salida.
5. Ejecuta `npm run ci`.
6. Abre un pull request con impacto, pruebas y compatibilidad.

## Estilo

- Go se formatea con `gofmt`.
- TypeScript, JSON, Markdown y YAML siguen `.prettierrc`.
- El código y sus comentarios se escriben en inglés.
- La documentación operativa se mantiene en español.
- Las cantidades monetarias usan enteros; no introduzcas coma flotante.
- Los puntos básicos deben validarse dentro de `[0, 10_000]`.

## Pruebas

```bash
npm ci
npm test
npm run ci
```

Un cambio de aritmética requiere casos de cero, límite, redondeo y overflow. Un
cambio de estado requiere transición válida, transición rechazada y proyección
JSON. Un cambio del cliente requiere una prueba de extremo a extremo contra el
binario real.

## Pull request

Incluye:

- problema operativo;
- decisión técnica;
- compatibilidad del esquema;
- comandos ejecutados;
- resultado en Ubuntu y Windows;
- actualización de documentación;
- estrategia de reversión.

No incluyas secretos, datos de terceros ni artefactos generados en el commit.

## Commits

Usa mensajes imperativos y concretos, por ejemplo:

```text
Add deterministic liquidity horizons
Document settlement integration contract
Harden release reference verification
```
