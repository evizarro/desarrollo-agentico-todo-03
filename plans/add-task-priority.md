# Feature: añadir prioridad a las tareas

## Encargo y criterios de aceptación

Encargo (literal): "añadir prioridad a las tareas".

Criterios de aceptación (explícitos derivados del encargo + inferencias necesarias para que sea implementable; las inferencias se marcan como tales):

- Cada tarea tiene una prioridad con uno de tres valores: `low`, `medium`, `high`. *(Inferencia: el encargo no especifica la escala; se elige un conjunto cerrado de 3 niveles por ser el estándar más simple y suficiente. Etiquetas en UI en español: "Baja", "Media", "Alta".)*
- Al crear una tarea desde el formulario de la página principal, se puede elegir la prioridad; si no se elige explícitamente, se usa `medium` por defecto.
- Al editar una tarea, se puede cambiar su prioridad (igual que ya se puede cambiar el título).
- La lista de tareas (`/`) muestra visualmente la prioridad de cada tarea mediante una etiqueta de texto con color (rojo=alta, ámbar=media, verde=baja), siguiendo el patrón visual ya usado para el estado "Completada". *(Decisión confirmada por el usuario: etiqueta de texto con color, no solo texto plano.)*
- La lista de tareas se ordena automáticamente por prioridad: primero las de prioridad alta, luego media, luego baja; dentro de una misma prioridad se mantiene el criterio actual (fecha de creación ascendente). *(Decisión confirmada por el usuario: ordenar automáticamente, no solo mostrar el dato.)*
- Las tareas ya existentes en base de datos (filas sembradas u otras previas a la migración) quedan con `medium` tras aplicar la migración, sin intervención manual. *(Decisión confirmada por el usuario.)*
- Un valor de prioridad fuera del conjunto permitido se rechaza con error de validación (422), igual que ocurre hoy con un título vacío.

Fuera de alcance salvo que se pida explícitamente (ver sección Alcance): filtrar la lista por prioridad (ocultar/mostrar según nivel), exponer prioridad en una API JSON dedicada, notificaciones, colores configurables por el usuario.

## Comportamiento actual y propuesta

**Actual**: `models/task.go` define `Task` con `ID`, `Title`, `Completed`, `CreatedAt`, `UpdatedAt`. No existe ningún concepto de prioridad en modelo, migraciones, rutas, templates, CSS ni tests. El único precedente de "editar un campo" es el título (commit `59ea8a9`, que añadió `TasksEdit`/`TasksUpdate` + `templates/tasks/edit.plush.html`), pero ese commit no tocó el modelo ni las migraciones porque el título ya existía desde la creación de la tabla. No hay ningún precedente en el repo de añadir una columna nueva a una tabla existente (`migrations/` solo contiene la migración de creación de `tasks` y el seed).

**Propuesto**: añadir una columna `priority` (string, NOT NULL, default `'medium'`) a `tasks`, un campo `Priority` en el modelo con validación de pertenencia al conjunto `{low, medium, high}`, extender `TasksCreate`/`TasksUpdate` para leerlo del formulario, y añadir el control de selección y el badge visual en `index.plush.html` y `edit.plush.html`, reutilizando exactamente los patrones ya existentes (bind seguro en `TasksUpdate` vía struct anónimo, clases CSS condicionales como `.task-completed`, mensajes de error vía `errors.Get(...)`).

## Alcance

**Incluido**:
- Columna `priority` en `tasks` vía migración nueva (up/down).
- Campo `Priority` en `models.Task`, con validación de valores permitidos.
- Selección de prioridad obligatoria en el formulario de creación (`index.plush.html`, `<select>` con `medium` preseleccionada) y en el de edición (`edit.plush.html`).
- Visualización de la prioridad en la lista de tareas (badge de texto con color).
- Ordenación de `TasksIndex` por prioridad (alta → media → baja) y, dentro de cada nivel, por fecha de creación ascendente como hoy.
- Normalización a `medium` cuando no se recibe prioridad explícita (defensivo, p. ej. llamadas directas al endpoint sin pasar por el `<select>`).
- Tests de modelo (validación de valores) y de acciones (create/update/index, incluyendo el nuevo orden) cubriendo el nuevo campo.
- Ajuste de estilos (`app.css`) para los badges de prioridad (con color por nivel) y bump de la versión de asset (`?v=`) en `application.plush.html`.

**Excluido** (decisión explícita para mantener el cambio vertical mínimo; se puede plantear como encargo separado):
- Filtrar la lista de tareas por prioridad (mostrar/ocultar según nivel); solo se reordena, no se oculta nada.
- Cambiar el comportamiento de `TasksToggle` o `TasksDestroy`.
- Tocar `public/assets/app.js`: no hay interacción dinámica nueva (el `<select>` se envía con el formulario normal, igual que el título; el reordenamiento ocurre en el servidor al recargar la lista, no en el cliente).
- Exponer la prioridad en un endpoint JSON específico (no existe ninguno hoy; los `json` tags del modelo se mantienen por consistencia pero no se añade una API).
- Migrar o re-sembrar datos con prioridades variadas en `20260901000001_seed_tasks.up.sql`: no se toca esa migración (ver Riesgos); el valor por defecto de columna basta para que las filas existentes queden en `medium`.

## Diseño por capas

### Persistencia y migración

Nueva migración `migrations/20260901000002_add_priority_to_tasks.up.fizz` / `.down.fizz` (el prefijo numérico debe ser posterior al último existente, `20260901000001`; usar timestamp real al crear el archivo si se prefiere seguir la convención de fecha-hora en vez de un contador manual — en este repo el patrón visto es contador manual tipo versión, no timestamp real, así que se mantiene la misma convención).

Up (sintaxis fizz estándar de `gobuffalo/fizz`; **no verificada contra el código fuente de `github.com/gobuffalo/fizz v1.14.4` fijado en `go.mod` de este repo** — antes de dar el plan por ejecutable hay que confirmarla, p. ej. revisando `$(go env GOPATH)/pkg/mod/github.com/gobuffalo/fizz@v1.14.4` o la documentación de Buffalo, o simplemente comprobando que `docker compose up --build` aplica la migración sin error):

```
add_column("tasks", "priority", "string", {"size": 10, "default": "medium", "null": false})
```

Down:

```
drop_column("tasks", "priority")
```

Esto no requiere tocar `migrations/embed.go` (el `//go:embed *.fizz *.sql` recoge los archivos nuevos automáticamente) ni `migrations/20260901000001_seed_tasks.up.sql` (el `DEFAULT 'medium'` de la columna rellena las filas sembradas sin reescribir ese INSERT).

**Importante (riesgo de diseño, no de sintaxis)**: el default de columna en Postgres solo se aplica cuando una sentencia SQL omite la columna. El ORM Pop, al mapear el struct `Task` completo, normalmente incluye todos los campos mapeados en el `INSERT`, incluido `Priority` con su valor-cero de Go (`""` para un `string` vacío) si no se asigna explícitamente antes de llamar a `tx.ValidateAndCreate`. Por tanto, **no basta con el default de columna para que las tareas creadas desde la app obtengan `medium`**; hay que normalizar el valor en la capa de acción (ver más abajo) antes de crear. El default de columna sigue siendo útil para: (a) las filas sembradas por SQL crudo (que si omiten la columna, sí reciben el default), y (b) cualquier inserción futura por SQL directo fuera del ORM.

### Modelo — `models/task.go`

- Añadir constantes para el conjunto cerrado de valores, p. ej.:
  ```go
  const (
  	PriorityLow    = "low"
  	PriorityMedium = "medium"
  	PriorityHigh   = "high"
  )
  ```
- Añadir campo al struct: `Priority string `json:"priority" db:"priority" form:"priority"`` (el tag `form:"priority"` es seguro en creación porque `TasksCreate` bindea sobre un `*models.Task{}` nuevo; en edición no importa porque `TasksUpdate` usa un struct anónimo intermedio, no bindea directo sobre el `Task` cargado — mismo patrón que `Title` hoy).
- Extender `Validate(tx *pop.Connection)`: si `t.Priority` no es `""` (ver nota sobre quién la normaliza) comprobar que pertenece a `{low, medium, high}`; si no, `verrs.Add("priority", "La prioridad debe ser baja, media o alta.")`.
- Definir dónde se normaliza el valor por defecto `medium` cuando llega vacío: se hace en la capa de acción (`actions/tasks.go`), no en `Validate`, para que `Validate` siga siendo una función pura de verificación (igual que hace con `Title` vía `strings.TrimSpace` en la propia acción antes de llamar a los métodos de persistencia — revisar el patrón exacto en `TasksCreate`/`TasksUpdate` actuales antes de implementar, para no introducir un estilo distinto).

### Rutas y acciones — `actions/tasks.go`, `actions/app.go`

No se añaden rutas nuevas; se reutilizan `POST /tasks` (`TasksCreate`) y `PUT /tasks/{task_id}` (`TasksUpdate`).

- `TasksCreate`: tras el bind a `task := &models.Task{}` y el trim del título, si `task.Priority == ""` asignar `models.PriorityMedium`. Igual que hoy valida con `tx.ValidateAndCreate(task)`.
- `TasksUpdate`: ampliar el `struct` anónimo de entrada para incluir `Priority string \`form:"priority" json:"priority"\`` junto a `Title`. Tras el bind, si `input.Priority == ""` usar `models.PriorityMedium` (o, alternativamente, exigir que el formulario siempre envíe un valor válido porque el `<select>` tendrá a `Media` como opción seleccionada por defecto — decidir en implementación cuál de las dos variantes es más consistente con cómo se trata `Title`, y documentarlo en el commit). Asignar `task.Priority = input.Priority` antes de `tx.ValidateAndUpdate(task)`.
- `TasksIndex`: cambiar el `tx.Order("created_at asc, id asc")` actual para ordenar primero por prioridad y, dentro de cada nivel, mantener el criterio actual. Al ser `priority` una columna de texto (`low`/`medium`/`high`), un `ORDER BY priority` alfabético no produce el orden deseado (alta→media→baja); hay que forzar el orden con una expresión `CASE` en el propio `Order`, p. ej.:
  ```go
  tx.Order("CASE priority WHEN 'high' THEN 0 WHEN 'medium' THEN 1 WHEN 'low' THEN 2 ELSE 3 END, created_at asc, id asc")
  ```
  (Pop pasa la cadena de `Order` directamente al `ORDER BY` de la consulta SQL, igual que ya hace con `"created_at asc, id asc"`, así que una expresión `CASE` es válida siempre que la sintaxis SQL sea correcta para Postgres — verificar con una consulta manual o con el propio test de `TasksIndex` antes de dar por bueno el comportamiento.)
- `TasksToggle`/`TasksDestroy`: sin cambios.

### Renderizado — templates y estilos

- `templates/tasks/index.plush.html`:
  - En el formulario de creación (líneas ~58-80 actuales), añadir un `<select name="priority">` con opciones `low`/`medium`/`high` (texto visible "Baja"/"Media"/"Alta"), con `medium` como `selected` por defecto.
  - En el bloque que itera `tasks` (donde hoy se muestra `task-status` para `Completed`), añadir un `<span class="task-priority task-priority-<%= t.Priority %>">` con el texto correspondiente ("Baja"/"Media"/"Alta"), siguiendo el mismo patrón condicional que `task-completed`.
- `templates/tasks/edit.plush.html`: añadir el mismo `<select name="priority">` que en creación, preseleccionando `task.Priority`, con su bloque de error asociado (`errors.Get("priority")`) igual que el bloque `title-error` existente.
- `public/assets/app.css`: añadir clases `.task-priority` (badge base) y `.task-priority-high` (rojo), `.task-priority-medium` (ámbar), `.task-priority-low` (verde), siguiendo el patrón visual de `.task-completed`/`.task-status` ya presente (mismo tipo de fondo/borde suave, no colores saturados, para mantener la coherencia visual del rediseño reciente del listado).
- `templates/application.plush.html`: incrementar `?v=3` → `?v=4` en el enlace a `app.css` (y el de `app.js` solo si finalmente se toca, lo cual no está previsto).

### Datos de prueba

- `fixtures/tasks.toml`: no requiere cambios obligatorios (el valor por defecto de columna cubre las filas que no especifiquen `priority`); si algún test nuevo necesita fixtures con prioridades distintas, se crean/actualizan en el propio test siguiendo el patrón de `ms.DB.ValidateAndCreate` visto en `models/task_test.go`, no editando el fixture compartido salvo que sea estrictamente necesario.

## Archivos relevantes

| Ruta | Responsabilidad en este cambio |
|---|---|
| `migrations/20260901000002_add_priority_to_tasks.up.fizz` (nuevo) | Añade la columna `priority` con default `medium` |
| `migrations/20260901000002_add_priority_to_tasks.down.fizz` (nuevo) | Revierte la columna |
| `models/task.go` | Campo `Priority`, constantes de valores permitidos, validación |
| `models/task_test.go` | Tests de validación del nuevo campo y de persistencia |
| `actions/tasks.go` | `TasksCreate`/`TasksUpdate`: leer, normalizar y persistir `priority` |
| `actions/tasks_test.go` | Tests de create/update/index cubriendo `priority` |
| `templates/tasks/index.plush.html` | Selector de prioridad en alta + badge de prioridad en el listado |
| `templates/tasks/edit.plush.html` | Selector de prioridad en edición + error asociado |
| `templates/application.plush.html` | Bump de `?v=` del CSS |
| `public/assets/app.css` | Estilos de los badges de prioridad |
| `fixtures/tasks.toml` | Sin cambios obligatorios; revisar si algún test nuevo lo requiere |

## Implementación paso a paso

1. Verificar la sintaxis exacta de `add_column`/`drop_column` en `gobuffalo/fizz v1.14.4` (fuente del paquete o documentación oficial de esa versión) antes de escribir la migración, en vez de asumir la sintaxis propuesta en este plan.
2. Crear la migración `20260901000002_add_priority_to_tasks.{up,down}.fizz` con la columna `priority` (string, tamaño razonable p. ej. 10, `NOT NULL DEFAULT 'medium'`).
3. Añadir a `models/task.go` las constantes `PriorityLow/Medium/High`, el campo `Priority` en el struct `Task`, y la comprobación de pertenencia al conjunto permitido dentro de `Validate`.
4. Actualizar `actions/tasks.go`:
   - `TasksCreate`: normalizar `task.Priority` a `models.PriorityMedium` cuando llegue vacío, antes de `tx.ValidateAndCreate`.
   - `TasksUpdate`: añadir `Priority` al struct de entrada, normalizar igual que en creación, asignar a `task.Priority` antes de `tx.ValidateAndUpdate`.
   - `TasksIndex`: cambiar el `Order` de la consulta para ordenar por prioridad (alta→media→baja) y luego por fecha de creación, según la expresión `CASE` descrita arriba.
5. Actualizar `templates/tasks/index.plush.html` (select obligatorio en el form de alta, preseleccionado en "Media" + badge de color en el listado) y `templates/tasks/edit.plush.html` (select en edición + error).
6. Añadir las clases de badge con color en `public/assets/app.css` y subir la versión del asset en `templates/application.plush.html`.
7. Ejecutar `docker compose up -d --build` (o `go run` local con Postgres disponible) y comprobar en los logs que la migración nueva se aplica sin error (`info - Successfully applied N migrations.`), igual que se vio al arrancar el proyecto por primera vez.
8. Probar manualmente en el navegador: crear tareas con cada prioridad y comprobar que la lista las reordena (alta arriba); editar la prioridad de una tarea y comprobar que cambia de posición y de color; confirmar que las tareas sembradas aparecen como "Media".
9. Añadir/actualizar tests (ver siguiente sección) y ejecutarlos.

## Tests y validación

- `models/task_test.go`:
  - Prioridad válida (`low`/`medium`/`high`) pasa validación.
  - Prioridad con valor arbitrario (p. ej. `"urgente"`) falla validación con error en el campo `priority`.
  - Persistencia: crear una tarea con `Priority: models.PriorityHigh`, actualizarla a `PriorityLow`, recargar y comprobar el valor (mismo patrón que `Test_Task_Toggle_Persists`).
- `actions/tasks_test.go`:
  - `TasksCreate` sin enviar `priority` en el POST crea la tarea con `medium`.
  - `TasksCreate` enviando una prioridad válida la persiste.
  - `TasksCreate`/`TasksUpdate` enviando una prioridad inválida devuelve 422 y el error es visible en la respuesta (igual que el test actual de título vacío).
  - `TasksUpdate` cambia la prioridad sin alterar `title`, `completed`, `id` ni `created_at` (extender el test existente que ya comprueba preservación de esos campos).
  - `TasksIndex` renderiza el badge/texto de prioridad para una tarea sembrada o creada en el test (comprobar presencia del texto "Baja"/"Media"/"Alta" en el HTML de respuesta, siguiendo el patrón de aserciones sobre el HTML ya usado en los tests existentes).
  - `TasksIndex` devuelve las tareas en el orden alta→media→baja: crear en el test tres tareas en orden baja, alta, media (en ese orden de creación) y comprobar que en el HTML de respuesta la de prioridad alta aparece antes que la media, y esta antes que la baja (comparando posiciones de substring o usando los helpers de parsing HTML que ya use el test suite, según lo que se vea al implementar).
- Comandos de validación (requieren PostgreSQL disponible y migraciones de test aplicadas, como ya documenta la skill `/feature`):
  - `gofmt -l .`
  - `go vet ./...`
  - `go build ./...`
  - `go test -p 1 ./...`

## Riesgos y recuperación

- **Sintaxis de fizz no verificada**: si `add_column` no es la forma correcta en `fizz v1.14.4`, la migración fallará al arrancar la app (`Migrate()` se ejecuta en el boot) y el contenedor `app` no quedará `healthy`. Mitigación: verificar la sintaxis antes de implementar (paso 1); si falla igualmente, corregir la migración y relanzar `docker compose up --build` — no afecta a datos porque es una migración aditiva sobre una tabla pequeña de desarrollo.
- **Zero-value de Pop en el INSERT**: si se omite la normalización en la capa de acción (`TasksCreate`/`TasksUpdate`) confiando solo en el default de columna, las tareas creadas desde la UI quedarán con `priority = ""` en vez de `medium`, y la validación las rechazará (si `Validate` exige pertenencia al conjunto) o las dejará en un estado inconsistente (si no la exige). Mitigación: la normalización explícita en la acción (pasos 4) y un test que cubra explícitamente "crear sin priority → queda medium" son obligatorios, no opcionales.
- **Columna `NOT NULL` sin default en el momento de aplicar si se omite `"default"` en la migración**: si la migración añade la columna como `NOT NULL` sin `DEFAULT`, Postgres la rechazará sobre una tabla con filas existentes (las sembradas). Mitigación: no quitar el `"default": "medium"` de la migración.
- **Fixture desactualizado**: si en el futuro se añade una comprobación estricta que dependa de un valor de `priority` concreto en `fixtures/tasks.toml` sin haberlo actualizado, los tests que usan `as.LoadFixture("tres tareas")` fallarán o pasarán con falsos positivos. Mitigación: si se decide fijar prioridades concretas en el fixture, hacerlo en el mismo cambio que el test que las necesita, no por separado.
- **Asset CSS cacheado**: si se olvida subir `?v=` en `application.plush.html` al cambiar `app.css`, navegadores con el asset cacheado no verán los nuevos estilos de badge. Mitigación: paso 6 explícito.
- **Orden por `CASE` en `tx.Order`**: si la cadena pasada a `Order` no es SQL válido para Postgres (error de sintaxis en el `CASE`) o si Pop antepone el nombre de tabla/alias de forma que rompe la expresión, `TasksIndex` fallará en tiempo de ejecución. Mitigación: probar la consulta manualmente (o vía el test de orden añadido) antes de dar el paso por completado; si falla, alternativa de bajo riesgo es añadir una columna auxiliar `priority_rank int` calculada en Go al leer las tareas (ordenar en memoria en el handler en vez de en SQL) sin tocar la migración.
