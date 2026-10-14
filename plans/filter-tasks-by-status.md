# Feature: Filtrar la lista de tareas por estado (todas, pendientes, completadas)

## Encargo y criterios de aceptación

Encargo (issue #4): añadir un selector (pestañas o enlaces) junto a la búsqueda para ver solo las
tareas pendientes, solo las completadas, o todas (como ahora). Debe combinarse con la búsqueda por
texto ya existente, sin que una funcionalidad rompa la otra.

Criterios de aceptación derivados:

- Explícitos (del texto de la issue):
  - Existe un selector visible junto al buscador con tres opciones: "Todas", "Pendientes",
    "Completadas".
  - Elegir una opción filtra la lista de tareas mostrada por su estado (`completed`).
  - El filtro de estado y la búsqueda por texto (`q`) funcionan combinados: buscar dentro de
    "Pendientes" solo debe devolver pendientes que coincidan con el texto, y viceversa.
  - El comportamiento actual (ver todas, buscar por texto) se mantiene cuando no se elige ningún
    filtro de estado explícito.
- Inferidos (necesarios para que el criterio anterior sea coherente con el resto de la app, no
  pedidos literalmente en la issue):
  - El filtro de estado se expresa en la URL (parámetro de query), igual que `q`, para que sea
    enlazable, compartible y sobreviva a recargas — igual patrón que la búsqueda existente.
  - El filtro de estado debe conservarse al completar/reabrir (`toggle`) o eliminar (`destroy`) una
    tarea, igual que ya ocurre con `q` (`redirectWithQuery`), para no perder el contexto tras una
    acción.
  - Un valor de filtro desconocido o ausente se trata como "todas" (degradación segura, no error).
  - Los mensajes de estado vacío ("No hay tareas...") deben seguir teniendo sentido cuando el vacío
    es consecuencia del filtro de estado (p. ej. no hay pendientes) y no solo de la búsqueda.

## Comportamiento actual y propuesta

Actual: `TasksIndex` (`actions/tasks.go:20`) solo admite el parámetro `q` para filtrar por título
(`ILIKE`). No existe ningún control de estado; la plantilla `templates/tasks/index.plush.html`
solo muestra el buscador. `redirectWithQuery` (`actions/tasks.go:163`) reconstruye `/` o
`/?q=...` tras `toggle`/`destroy`, pero no conoce ningún otro parámetro.

Propuesta: añadir un parámetro de query `status` con tres valores válidos (`all` por defecto,
`pending`, `completed`). `TasksIndex` lo combina con el filtro de texto existente mediante
`tx.Where` encadenado (AND lógico, igual que Pop ya hace con múltiples `.Where`). La plantilla
añade una barra de enlaces ("Todas" / "Pendientes" / "Completadas") que preserva la búsqueda activa
al cambiar de pestaña, y marca la pestaña activa con `aria-current="page"`. Los formularios de
`toggle` y `destroy`, y el propio buscador, llevan un campo oculto `status` para que la acción o la
búsqueda siguiente no pierda el filtro activo. Se generaliza `redirectWithQuery` a una función que
reconstruye la URL con `q` y `status`.

## Alcance

Incluido:
- Parámetro `status` en `TasksIndex`, con normalización a `all|pending|completed`.
- Combinación de `status` y `q` en la misma consulta.
- Enlaces/pestañas de estado en `templates/tasks/index.plush.html`, con estado activo accesible.
- Preservación de `status` (y `q`) en los formularios de `toggle` y `destroy`, y en el buscador.
- Mensajes de "lista vacía" adaptados al caso en que el filtro de estado (sin búsqueda) no tiene
  resultados.
- Estilos mínimos en `public/assets/app.css` para las pestañas, reutilizando clases ya existentes
  (`.button`) más un estado activo.
- Tests en `actions/tasks_test.go` que cubran el filtro solo, la búsqueda sola, y la combinación de
  ambos, además de la preservación tras `toggle`/`destroy`.

Excluido:
- Cualquier cambio en el modelo `Task` o en migraciones: el estado ya existe como columna
  `completed`, no se necesita persistencia nueva.
- Paginación o contadores por estado (p. ej. "3 pendientes, 2 completadas" simultáneos); el
  contador existente (`list-count`) se mantiene mostrando el total de la lista filtrada actual.
- Preservar el filtro de estado al crear una tarea nueva (`TasksCreate`): hoy ese flujo ya
  descarta `q` en caso de error de validación (`actions/tasks.go:67`) y recarga la lista completa;
  se mantiene ese mismo comportamiento para `status` por consistencia, sin extender el alcance.
- Guardar el filtro de estado en sesión o cookie: vive solo en la URL, igual que `q`.

## Diseño por capas

### Acción (`actions/tasks.go`)
- Nuevas constantes de estado (`StatusAll`, `StatusPending`, `StatusCompleted`) y una función
  `normalizeStatus(raw string) string` que solo acepta `pending`/`completed` y degrada a `all` en
  cualquier otro caso (incluida cadena vacía).
- `TasksIndex` lee `status := normalizeStatus(c.Param("status"))`, añade `.Where("completed = ?",
  false/true)` según el valor (sin cláusula extra si es `all`), y expone al contexto: `status` y
  tres URLs precalculadas (`statusAllURL`, `statusPendingURL`, `statusCompletedURL`) para que la
  plantilla no tenga que construir query strings.
- `TasksCreate`, en su rama de error de validación, fija `status = StatusAll` (igual que ya fija
  `query = ""`) y calcula las mismas tres URLs con esos valores, para que la plantilla compartida
  siga funcionando.
- `redirectWithQuery` se renombra a `redirectToList` y añade `status` (leído y normalizado desde
  `c.Param("status")`) a la URL de vuelta, usando `url.Values` para construir la query string
  (sustituye la construcción manual actual, que solo sabía de `q`).
- Nueva función auxiliar `taskListURL(status, query string) string` usada tanto por `TasksIndex`/
  `TasksCreate` (para las URLs de las pestañas) como por `redirectToList` (para la URL de
  redirección), evitando duplicar la lógica de construcción de query string.

### Plantilla (`templates/tasks/index.plush.html`)
- Se añade un `<nav class="status-tabs" aria-label="Filtrar por estado">` con tres `<a>` (Todas /
  Pendientes / Completadas) justo debajo del formulario de búsqueda, usando las URLs precalculadas
  y marcando `aria-current="page"` en la pestaña que coincide con `status`.
- El formulario de búsqueda añade `<input type="hidden" name="status" value="<%= status %>">` para
  que al buscar se mantenga el filtro de estado activo.
- Los formularios de `toggle` y de `destroy` añaden `<input type="hidden" name="status"
  value="<%= status %>">` junto al `q` oculto ya existente.
- Los mensajes de estado vacío distinguen cuatro casos en vez de dos: sin resultados de búsqueda
  (igual que hoy), sin tareas en absoluto (`status == "all"` y sin `query`), sin pendientes
  (`status == "pending"` y sin `query`), sin completadas (`status == "completed"` y sin `query`).

### Estilos (`public/assets/app.css`)
- Nueva regla `.status-tabs` (flex, gap, `margin-top`) a continuación de `.task-search` (línea
  ~116), reutilizando la clase `.button` ya existente para cada enlace y añadiendo un selector
  `.status-tabs a[aria-current="page"]` con los colores de `.button-primary` para marcar la
  pestaña activa.

## Archivos relevantes

- `actions/tasks.go` — `TasksIndex`, `TasksCreate`, `redirectWithQuery`→`redirectToList`, nuevas
  constantes/helpers de estado.
- `templates/tasks/index.plush.html` — pestañas de estado, campos ocultos `status`, mensajes de
  vacío.
- `public/assets/app.css` — estilos de `.status-tabs`.
- `actions/tasks_test.go` — tests nuevos de filtrado y de preservación del filtro.

## Implementación paso a paso

1. En `actions/tasks.go`, añadir las constantes de estado y `normalizeStatus`:
   ```go
   const (
       StatusAll       = "all"
       StatusPending   = "pending"
       StatusCompleted = "completed"
   )

   func normalizeStatus(raw string) string {
       switch raw {
       case StatusPending, StatusCompleted:
           return raw
       default:
           return StatusAll
       }
   }
   ```
2. Añadir `taskListURL`:
   ```go
   func taskListURL(status, query string) string {
       params := url.Values{}
       if status != StatusAll {
           params.Set("status", status)
       }
       if query != "" {
           params.Set("q", query)
       }
       if len(params) == 0 {
           return "/"
       }
       return "/?" + params.Encode()
   }
   ```
3. Actualizar `TasksIndex` para leer y normalizar `status`, encadenar el `.Where` de estado tras el
   de búsqueda, y exponer `status` + las tres URLs de pestaña al contexto (reemplaza el bloque
   `q := tx.Order(...)` actual en `actions/tasks.go:25-39`).
4. Actualizar la rama de error de `TasksCreate` (`actions/tasks.go:60-71`) para fijar
   `status := StatusAll` y exponer las mismas tres claves de contexto que `TasksIndex`
   (`status`, `statusAllURL`, `statusPendingURL`, `statusCompletedURL`), con `query := ""` como ya
   hace.
5. Sustituir `redirectWithQuery` por `redirectToList`, construyendo la URL con `taskListURL` a
   partir de `strings.TrimSpace(c.Param("q"))` y `normalizeStatus(c.Param("status"))`; actualizar
   las dos llamadas en `TasksToggle` y `TasksDestroy`.
6. En `templates/tasks/index.plush.html`:
   - Añadir el `<input type="hidden" name="status" ...>` al formulario de búsqueda.
   - Añadir el `<nav class="status-tabs">` con los tres enlaces, usando `statusAllURL`,
     `statusPendingURL`, `statusCompletedURL` y comparaciones `status == "..."` para
     `aria-current`.
   - Añadir el `<input type="hidden" name="status" ...>` a los formularios de `toggle` y de
     `destroy` (los dos `<form>` dentro del `for` de tareas).
   - Extender la rama de estado vacío (`len(tasks) == 0`) para cubrir los cuatro casos descritos
     arriba, con un texto claro por combinación (p. ej. "No tienes tareas pendientes." / "No
     tienes tareas completadas.").
7. En `public/assets/app.css`, añadir las reglas `.status-tabs` y
   `.status-tabs a[aria-current="page"]` cerca de `.task-search` (línea 116), siguiendo la
   paleta ya usada por `.button` / `.button-primary`.
8. Revisar que ningún otro lugar del código llame a `redirectWithQuery` por su nombre antiguo
   (`grep -rn redirectWithQuery`) antes de darlo por renombrado.

## Tests y validación

Añadir a `actions/tasks_test.go` (siguiendo el estilo de los tests de búsqueda ya existentes,
usando la fixture `tres tareas` cuando encaje o creando tareas sueltas con `as.DB.ValidateAndCreate`
cuando se necesite controlar `Completed` con precisión):

- `Test_TasksIndex_StatusPendingShowsOnlyPending`: con dos tareas, una completada y otra no,
  `/?status=pending` contiene el título de la pendiente y no el de la completada.
- `Test_TasksIndex_StatusCompletedShowsOnlyCompleted`: inverso del anterior.
- `Test_TasksIndex_StatusAllShowsEverything`: `/?status=all` (y `/` sin parámetro) muestra ambas.
- `Test_TasksIndex_UnknownStatusDefaultsToAll`: `/?status=algo-raro` se comporta como `status=all`.
- `Test_TasksIndex_StatusCombinesWithSearch`: tres tareas con títulos distintos y estados mixtos;
  `/?status=pending&q=<texto>` solo devuelve la que es pendiente *y* coincide con el texto.
- `Test_TasksIndex_EmptyState_NoPendingTasks` / `Test_TasksIndex_EmptyState_NoCompletedTasks`:
  todas las tareas completadas y se pide `/?status=pending` (o viceversa) → aparece el mensaje de
  vacío específico de ese estado, no el genérico de "no hay tareas todavía".
- `Test_TasksIndex_StatusTabsMarkActiveTab`: comprobar que la respuesta contiene
  `aria-current="page"` asociado al enlace correspondiente al `status` pedido (por ejemplo,
  comprobando que aparece una sola vez y en el tramo de HTML del enlace "Pendientes").
- `Test_TasksToggle_PreservesStatusAndQuery`: extender el test existente
  (`Test_TasksToggle_PreservesSearchQuery`, `actions/tasks_test.go:64`) para enviar también
  `status=pending` y comprobar que la redirección es `/?status=pending&q=pendiente` (o el orden que
  produzca `url.Values.Encode()`, verificar el valor real en vez de asumir orden alfabético si
  difiere).
- `Test_TasksDestroy_PreservesStatusAndQuery`: igual que el anterior para `TasksDestroy`.

Validación de comandos (requiere PostgreSQL arrancado y migraciones de test preparadas, como ya
exige la suite de `actions`):
- `gofmt -l .` (sin salida).
- `go vet ./...`
- `go build ./...`
- `go test -p 1 ./...`

## Riesgos y recuperación

- Riesgo: romper los tests existentes `Test_TasksToggle_PreservesSearchQuery` y
  `Test_TasksDestroy_PreservesSearchQuery` si `taskListURL`/`redirectToList` no reproduce
  exactamente `/?q=pendiente` cuando no se envía `status`. Mitigación: `normalizeStatus("")`
  devuelve `StatusAll`, y `taskListURL` omite el parámetro `status` cuando vale `StatusAll`, por lo
  que la URL resultante es idéntica a la actual en ausencia de ese parámetro.
- Riesgo: que `url.Values.Encode()` ordene las claves de forma distinta a la esperada por un test
  que compare la URL como cadena literal. Mitigación: los tests nuevos deben comprobar el valor
  exacto devuelto por la implementación (ejecutar el test primero y fijar la aserción al resultado
  real), no asumir un orden sin verificarlo.
- Riesgo: el filtro de estado por `.Where("completed = ?", ...)` interactúe mal con el `ORDER BY`
  de prioridad (`taskOrder`) si Pop antepone el `WHERE` de forma inesperada al encadenar múltiples
  `.Where`. Mitigación: Pop ya combina múltiples `.Where` con AND de forma documentada y es el
  mismo patrón que ya usa el filtro de texto; verificar con el test de combinación
  (`Test_TasksIndex_StatusCombinesWithSearch`) que el resultado es la intersección esperada.
- Recuperación: todos los cambios son aditivos sobre un flujo ya cubierto por tests; si algún test
  nuevo falla de forma no evidente, revertir solo la plantilla (la parte más propensa a errores de
  marcado Plush) y volver a aplicarla por partes (primero los campos ocultos, después las pestañas)
  para aislar el fallo.
