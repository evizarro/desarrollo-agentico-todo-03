---
name: adw
description: Orquesta el flujo completo de una tarea - planifica con /feature, lo implementa con /implement, y hace commit; en modo interactivo pide tu aprobación del plan, en modo issue de GitHub (desatendido) hace push y abre un PR
allowed-tools: Bash, Read, Glob, Grep, Write, Edit, Skill, AskUserQuestion
---

# Flujo ADW: planifica, revisa y ejecuta una tarea

Este flujo coordina dos skills que ya existen en `.claude/skills/` (`feature` e `implement`) y tiene **dos modos**, según la forma de `$ARGUMENTS`. No sustituye esas skills ni duplica sus reglas: se limita a encadenarlas y a adaptar el punto de revisión según si hay o no una persona delante.

## 0. Detecta el modo

- **Modo issue (desatendido)**: `$ARGUMENTS` empieza por una URL de issue de GitHub (`https://github.com/<owner>/<repo>/issues/<número>`), opcionalmente seguida de `--remote <nombre>` y `--base <rama>`. Si faltan, `--remote` es `origin` y `--base` es `main`. Así es como lo invoca `adws/trigger-github.sh` (ver `config/adw.yml`): sin nadie delante para responder preguntas.
- **Modo interactivo**: cualquier otro caso. Todo `$ARGUMENTS` es la descripción del encargo en lenguaje natural, y asumes que hay una persona en la conversación que puede responder.

Todo lo que sigue se aplica a ambos modos salvo que se indique lo contrario explícitamente para uno de ellos.

## 1. Obtén el encargo

- **Interactivo**: el encargo es `$ARGUMENTS` tal cual.
- **Issue**: ejecuta `gh issue view <issue_url> --json number,title,body,url`. El encargo para `/feature` es el título más el cuerpo de la issue. Guarda el número de issue (para la rama y el PR) y la URL (para comentar si hace falta).

## 2. Prepara la rama (solo modo issue)

- Comprueba `git status --short`. Si hay cambios sin commitear ajenos a este encargo, detente y dilo: no mezcles este trigger con otro trabajo en curso en el checkout.
- Actualiza la referencia del remoto (`git fetch <remote> <base>`) y crea una rama nueva a partir de ella: `git checkout -b adw/issue-<número> <remote>/<base>`.
- En modo interactivo no se crea ninguna rama: se trabaja sobre la rama actual, como hasta ahora.

## 3. Planifica

Invoca la skill `feature` pasándole el encargo obtenido en el paso 1 (herramienta Skill con `skill: feature`, `args: <encargo>`). Si no está disponible en la sesión, sigue manualmente `.claude/skills/feature/SKILL.md` con ese mismo argumento.

Esto aplica igual si el encargo es una funcionalidad nueva, un bug o una tarea de mantenimiento: `/feature` investiga el comportamiento actual frente al esperado y escribe un plan autocontenido en `plans/`, sin implementar nada todavía.

## 4. Revisa el plan

- **Interactivo**: lee el plan entero y preséntale al usuario un resumen breve (encargo y criterios, alcance incluido/excluido, archivos a tocar, pasos clave, riesgos). Pregunta explícitamente si se aprueba, se ajusta, o se descarta. No pases al paso 5 sin una confirmación explícita, por trivial que parezca el encargo.
  - Si pide ajustes, edita el plan ya existente en `plans/` (no lo reescribas desde cero) y vuelve a resumir hasta conseguir la aprobación.
  - Si se descarta, detente aquí y dile dónde quedó el plan.
- **Issue**: no hay nadie que pueda aprobar. Sigue directamente al paso 5 sin pedir confirmación — la revisión humana de este modo ocurre más tarde, sobre el Pull Request, no sobre el plan.

## 5. Implementa

Con el plan listo (aprobado en modo interactivo, o directamente en modo issue), invoca la skill `implement` pasándole la ruta del plan (herramienta Skill con `skill: implement`, `args: <ruta del plan>`). Si no está disponible, sigue manualmente `.claude/skills/implement/SKILL.md` con esa misma ruta.

Esto ejecuta el plan, añade o actualiza solo los tests que exige, y corre sus comandos de validación.

## 6. Revisa el resultado

- **Interactivo**: comprueba que el cambio funciona de verdad, no solo que los tests pasan — si el proyecto tiene un servidor o una app ejecutable, levántala (o reutiliza una instancia ya corriendo) y verifica a mano el comportamiento descrito en el encargo.
- **Issue**: no hay un navegador fiable en un proceso desatendido. Limita la revisión a las validaciones automatizadas que define el plan (tests, build, etc.). Si el plan exige una comprobación manual que no se puede automatizar, decláralo como limitación en el informe final; no la inventes ni la des por hecha en silencio.

Informa en ambos casos con el mismo formato que usa `implement` (archivos cambiados, tests añadidos o actualizados, cada comando ejecutado con su resultado, desviaciones o riesgos residuales).

## 7. Cierra: commit (y push + PR en modo issue)

**Antes de decidir nada en este paso, vuelve a comprobar en qué modo estás** (paso 0): tras planificar e implementar es fácil "olvidar" que esto empezó como una URL de issue y tratar el cierre como si hubiera alguien delante. Si `$ARGUMENTS` era una URL de issue, estás en **modo issue**, y el commit+push+PR de más abajo **no son opcionales ni requieren que nadie te lo pida**: pedir confirmación aquí es un error, porque en este modo no hay nadie que vaya a responder y el encargo se quedaría a medias, sin commitear, bloqueando además cualquier encargo futuro que comparta este mismo checkout (como ya le pasó a otra ejecución de este mismo flujo).

**Si la revisión no es satisfactoria** (alguna validación falla, o hay un bloqueo o ambigüedad real):

- **Interactivo**: informa del bloqueo con claridad; no hagas commit.
- **Issue**: no hagas commit ni push. Deja el trabajo tal cual quedó en el checkout local (no lo descartes, por si alguien quiere inspeccionarlo), vuelve a la rama `--base` original si es más seguro para no dejar el checkout a medio cambiar antes de la próxima ejecución del trigger, y comenta en la issue (`gh issue comment <issue_url> --body "..."`) explicando en qué paso se bloqueó y por qué. Detente ahí.

**Si la revisión es satisfactoria**:

- Ambos modos: comprueba `git status --short` y añade explícitamente solo los archivos que pertenecen a este encargo (el código que toca el plan más el propio plan en `plans/`), nunca con `git add -A` ni `git add .` a ciegas. Si algo no está claro sobre qué pertenece a este encargo, pregúntalo en modo interactivo; en modo issue, trátalo como ambigüedad bloqueante (ver arriba) en vez de adivinar. Escribe el mensaje de commit siguiendo el estilo ya usado en el historial del repo (`git log --oneline`).
- **Interactivo**: no hagas push, no hagas merge, no abras un pull request. Informa del hash y el resumen del commit.
- **Issue**: haz siempre, sin preguntar ni esperar confirmación, `git push <remote> adw/issue-<número>` y abre un PR con `gh pr create --base <base> --head adw/issue-<número> --title "..." --body "..."`, referenciando la issue en el cuerpo (p. ej. `Closes #<número>`) para que se cierre sola al fusionarse. Informa de la URL del PR creado.

## Límites

- Modo interactivo: nunca push, merge ni PR; nunca saltes el paso de aprobación del plan.
- Modo issue: nunca commitees ni hagas push si la revisión no pasa; nunca toques la rama `--base` directamente, todo el trabajo va en `adw/issue-<número>`; nunca resuelvas una ambigüedad bloqueante adivinando, coméntala en la issue y detente; y, si la revisión sí pasa, nunca pidas confirmación para commitear/pushear/abrir el PR — hazlo directamente, es el comportamiento esperado de este modo, no una opción.
- En ambos modos: no mezcles este encargo con otro trabajo sin commitear que ya hubiera en el repo antes de empezar.
