---
name: adw
description: Orquesta el flujo completo de una tarea - planifica con /feature, pide revisión y aprobación del plan, y lo implementa con /implement
allowed-tools: Bash, Read, Glob, Grep, Write, Edit, Skill, AskUserQuestion
---

# Flujo ADW: planifica, revisa y ejecuta una tarea

`$ARGUMENTS` debe contener la descripción del encargo (la tarea a resolver): una funcionalidad nueva, un bug o una tarea de mantenimiento, en lenguaje natural.

Este flujo coordina dos skills que ya existen en `.claude/skills/` (`feature` e `implement`) y añade un punto de revisión humana entre ambas. No sustituye esas skills ni duplica sus reglas: se limita a encadenarlas y a presentar el resultado intermedio para su aprobación antes de tocar código.

## 1. Planifica

Invoca la skill `feature` pasándole `$ARGUMENTS` como descripción del encargo (herramienta Skill con `skill: feature`, `args: $ARGUMENTS`). Si la skill no está disponible en la sesión actual (por ejemplo, porque el proyecto se acaba de clonar y las skills no se han recargado todavía), sigue manualmente las instrucciones de `.claude/skills/feature/SKILL.md` con ese mismo argumento, en vez de detenerte.

Esto aplica igual si el encargo es una funcionalidad nueva, un bug o una tarea de mantenimiento: `/feature` investiga el comportamiento actual frente al esperado y escribe un plan autocontenido en `plans/`, sin implementar nada todavía.

## 2. Resume el plan y pide aprobación

Cuando el plan esté escrito, léelo entero y preséntale al usuario un resumen breve (no el archivo completo) con, como mucho, estos puntos:

- Encargo y criterios de aceptación.
- Alcance incluido y excluido.
- Archivos que se van a tocar.
- Pasos clave de implementación.
- Riesgos o incertidumbres que señale el plan.

Pregunta explícitamente si el plan se aprueba tal cual, necesita ajustes, o se descarta. No pases a la fase de implementación sin una confirmación explícita del usuario, por trivial que parezca el encargo.

- Si pide ajustes, edita el plan ya existente en `plans/` para reflejarlos (no lo reescribas desde cero ni crees uno nuevo) y vuelve a resumir hasta conseguir la aprobación.
- Si se descarta, detente aquí y dile dónde quedó el plan por si quiere retomarlo más adelante.

## 3. Implementa

Con el plan aprobado, invoca la skill `implement` pasándole la ruta del plan (herramienta Skill con `skill: implement`, `args: <ruta del plan>`). Si no está disponible en la sesión, sigue manualmente `.claude/skills/implement/SKILL.md` con esa misma ruta como argumento.

Esto ejecuta el plan, añade o actualiza solo los tests que el plan exige, y corre sus comandos de validación.

## 4. Revisa el resultado

Tras implementar, comprueba que el cambio funciona de verdad y no solo que los tests pasan: si el proyecto tiene un servidor o una app ejecutable, levántala (o reutiliza una instancia ya corriendo) y verifica a mano el comportamiento descrito en el encargo, no solo el camino feliz.

Informa con el mismo formato que usa `implement` (archivos cambiados, tests añadidos o actualizados, cada comando ejecutado con su resultado, desviaciones o riesgos residuales) y añade qué comprobaste manualmente y qué observaste.

## 5. Haz commit

Si la revisión del paso anterior es satisfactoria (las validaciones pasan y la comprobación manual confirma el comportamiento esperado), haz commit de los cambios de este encargo:

- Comprueba `git status --short` y añade explícitamente solo los archivos que pertenecen a este plan (los que toca el código más el propio archivo en `plans/`), nunca con `git add -A` ni `git add .` a ciegas. Si hay otros cambios sin commitear de una tarea distinta, no los incluyas en este commit.
- Si algo no está claro (por ejemplo, qué archivos pertenecen a este encargo frente a trabajo previo sin commitear de otra tarea), pregúntalo antes de hacer `git add`; no asumas.
- Escribe el mensaje de commit siguiendo el estilo ya usado en el historial del repo (consúltalo con `git log --oneline`): un tipo breve al inicio (`feat`, `fix`, `chore`, etc.) seguido de una descripción corta centrada en el porqué del cambio.
- No hagas push, no hagas merge y no abras un pull request: el commit se queda en local. Informa del hash y el resumen del commit creado.

## Límites

- No hagas push, merge ni abras un pull request: queda fuera de este flujo. El commit del paso 5 sí forma parte del flujo.
- No saltes el paso de aprobación del plan, aunque el encargo parezca trivial o ya se haya discutido en la conversación.
- No mezcles este flujo con el trigger de issues de `adws/`: esta skill está pensada para uso interactivo con una persona delante, no para ejecución desatendida.
