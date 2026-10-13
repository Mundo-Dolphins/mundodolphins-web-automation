# mundodolphins-web-automation

[![CI](https://github.com/Mundo-Dolphins/mundodolphins-web-automation/actions/workflows/ci.yml/badge.svg)](https://github.com/Mundo-Dolphins/mundodolphins-web-automation/actions/workflows/ci.yml)
[![License: MIT](https://img.shields.io/badge/license-MIT-blue.svg)](LICENSE)

Librería Go (y binario de ejemplo) que mantiene los datos de una web estática sincronizados con
un **feed RSS de podcast** y con **posts de redes sociales**, y que puede abrir una pull request
con el resultado. Es la base de la automatización del sitio de [Mundo Dolphins](https://mundodolphins.es).

> Proyecto de la comunidad Mundo Dolphins. No está afiliado ni respaldado por ninguna de las
> plataformas mencionadas; los nombres se usan solo de forma descriptiva.

## Qué incluye y qué no

| Fuente | Cómo |
|---|---|
| Feed RSS de podcast | Estándar (RSS 2.0 + extensiones iTunes), cualquier plataforma que lo publique |
| Bluesky | API pública oficial (`public.api.bsky.app`), con RSS/oEmbed como alternativa |
| GitHub | API oficial para abrir PRs; `git` mediante `os/exec` |
| Páginas web de podcast | **No incluido.** Interfaz `podcast.PageScraper` |
| Instagram | **No incluido.** Interfaz `social.InstagramSource` |

La librería **no incluye ningún scraper de webs de terceros**: leer el HTML de una plataforma o
Instagram implica términos de uso que cada usuario debe valorar. Si necesitas esas fuentes,
implementa las interfaces en tu propio código; esa implementación es responsabilidad tuya.

## Paquetes

| Paquete | Para qué |
|---|---|
| `podcast` | Modelo `Episode`, lector RSS (`RSSParser`), reparto por temporadas y sincronización con `season_N.json` |
| `social` | Modelo `OutputPost`, lector de Bluesky (`Reader`) e interfaz `InstagramSource` |
| `cli` | Comandos `rss`, `social`, `social-autopr`, `checker` (cobra), inyectables con `cli.Options` |
| `autopr`, `gitops`, `github` | Rama de trabajo → commit → push → PR, con locks, reintentos y secretos redactados |
| `autoconfig`, `filelock`, `retryhttp` | Configuración por entorno, lock de fichero (solo Unix) y reintentos HTTP |

```bash
go get github.com/Mundo-Dolphins/mundodolphins-web-automation@latest
```

> **Estabilidad:** mientras la versión sea `v0.x`, la API puede cambiar entre versiones menores.
> Fija una versión concreta en tu `go.mod`.

## Binario de ejemplo

```bash
export RSS_FEED_URL=https://example.com/podcast/feed.xml
export SOCIAL_BSKY_PROFILES=alice.example.com

go run ./cmd/web-automation rss --data ./data
go run ./cmd/web-automation social --data ./data
```

`social-autopr` y `checker` además escriben en un clon del repositorio del sitio, empujan una rama
fija y abren (o actualizan) una PR; necesitan `GITHUB_OWNER`, `GITHUB_REPO`, `GITHUB_TOKEN` y
`REPO_PATH` (ver [.env.example](.env.example)).

## Aportar tus propias fuentes

```go
package main

import (
	"github.com/Mundo-Dolphins/mundodolphins-web-automation/cli"
	"github.com/Mundo-Dolphins/mundodolphins-web-automation/social"
)

// myInstagram implementa social.InstagramSource con la fuente que tú elijas.
type myInstagram struct{}

func (myInstagram) FetchProfile(profile string, known []string) ([]social.OutputPost, error) {
	// ... devuelve los posts nuevos con SType: social.Instagram
	return nil, nil
}

func main() {
	root := cli.New(cli.Options{
		FeedURL:           "https://example.com/podcast/feed.xml",
		Social:            social.Reader{Instagram: myInstagram{}},
		InstagramProfiles: []string{"your_profile"},
		BlueSkyProfiles:   []string{"alice.example.com"},
		// Preferir una URL de audio sobre otra para el mismo episodio (opcional).
		PreferAudio: func(url string) bool { return false },
	})
	// root.AddCommand(...) para añadir tus propios comandos (por ejemplo, un scraper de páginas).
	cli.Execute(root)
}
```

Los episodios se asignan a temporadas con `podcast.SeasonStarts` (inicio del año de liga de la
NFL de cada temporada); hay que **ampliarlo cada año**.

## Desarrollo

```bash
make ci          # fmt-check + vet + test (con -race) + lint
make coverage
```

Los tests no usan red real (servidores `httptest`, `httpmock` y runners falsos de git).

## Seguridad

Ver [SECURITY.md](SECURITY.md). El token de GitHub va solo en el entorno; los errores de `git` y de
la API de GitHub se registran con los secretos redactados.

## Licencia

[MIT](LICENSE)
