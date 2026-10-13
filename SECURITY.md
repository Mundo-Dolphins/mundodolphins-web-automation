# Seguridad

## Reportar una vulnerabilidad

Usa **Security > Report a vulnerability** (aviso privado de GitHub) en este repositorio.
No abras un issue publico con detalles de una vulnerabilidad ni pegues tokens en issues o PRs.

## Buenas practicas al usar el bot

- `GITHUB_TOKEN` va **solo** en variables de entorno o en un `.env` fuera del repositorio.
- Si un token se filtra, revócalo en GitHub (Settings > Developer settings) y genera uno nuevo con los permisos mínimos (solo el repo del sitio).
- Los ficheros de estado (`data/`) son públicos en el sitio web, pero el token de GitHub nunca debe versionarse.
