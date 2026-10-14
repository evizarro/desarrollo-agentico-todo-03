# Feature: contador de tareas pendientes vs completadas

## Encargo y criterios de aceptación

Encargo (issue #1, `evizarro/desarrollo-agentico-todo-03`): "Añadir contador de tareas pendientes vs completadas. Mostrar en la cabecera de la lista de tareas cuántas están pendientes y cuántas completadas, además del total que ya se muestra hoy (ej.: «3 tareas en tu lista · 2 pendientes · 1 completada»)."

Criterios de aceptación:

- La cabecera de `/` muestra, además del total ya existente, cuántas tareas están pendientes y cuántas completadas.
- El desglose usa singular/plural correcto ("1 pendiente" / "2 pendientes", "1 completada" / "2 completadas"), igual que ya hace el contador de total ("1 tarea" / "2 tareas").
- Si hay una búsqueda activa (`?q=`), el desglose refleja los resultados filtrados, no el total absoluto de tareas. *(Inferencia: la issue no contempla la búsqueda porque no existía cuando se redactó; pero el total ya se adapta a la búsqueda hoy, así que el desglose debe seguir el mismo criterio por consistencia.)*
- Con cero tareas (lista vacía o búsqueda sin resultados), el desglose puede mostrar "0 pendientes · 0 completadas" sin caso especial. *(Inferencia: más simple que ocultarlo condicionalmente y no contradice el ejemplo de la issue.)*

## Comportamiento actual y propuesta

**Actual**: `TasksIndex` (`actions/tasks.go`) ya calcula `tasks` (filtradas por `q` si hay búsqueda) y expone `len(tasks)` a `templates/tasks/index.plush.html`, que lo usa en `<p class="list-count">` para mostrar "N tareas en tu lista" o "N resultados para «q»". No existe ningún desglose por estado; `models.Task.Completed` es el único dato de estado por tarea.

**Propuesto**: en `TasksIndex`, recorrer la misma colección `tasks` ya cargada (no hacer una consulta nueva) para contar cuántas tienen `Completed == true`, exponer `completedCount` y `pendingCount` (`len(tasks) - completedCount`) a la plantilla, y añadir el desglose al `<p class="list-count">` ya existente, reutilizando el mismo patrón condicional singular/plural.

## Alcance

**Incluido**:
- Cálculo de `pendingCount`/`completedCount` en `TasksIndex`, a partir de la colección ya filtrada por búsqueda.
- Mostrar el desglose en la cabecera, en ambos casos (con y sin búsqueda activa).
- Tests que cubran el conteo con una mezcla de tareas pendientes/completadas, y con una búsqueda activa.

**Excluido**:
- Cambiar `TasksCreate` (su rama de error también renderiza `index.plush.html`, pero crear una tarea nunca deja la vista en un estado donde el desglose sea observable de forma distinta al de `TasksIndex`; aun así, para que la plantilla no falle al faltar la variable, hay que exponerla igual que se hizo con `query`).
- Filtrar o enlazar el desglose (p. ej. que "pendientes" sea un enlace que filtre la lista): la issue solo pide mostrar el número, no interactuar con él.
- Tocar el modelo o las migraciones: `Completed` ya existe, no hace falta ningún dato nuevo en base de datos.

## Diseño por capas

### Rutas y acciones — `actions/tasks.go`

- `TasksIndex`: tras obtener `tasks`, calcular:
  ```go
  completedCount := 0
  for _, t := range tasks {
      if t.Completed {
          completedCount++
      }
  }
  pendingCount := len(tasks) - completedCount
  ```
  y exponerlos con `c.Set("completedCount", completedCount)` / `c.Set("pendingCount", pendingCount)`.
- `TasksCreate`, rama de error (`verrs.HasAny()`): exponer igualmente `pendingCount`/`completedCount` calculados sobre la misma `tasks` que ya recarga esa rama, por el mismo motivo por el que ya expone `query` ahí (evitar que la plantilla falle por falta de la variable).

### Renderizado — `templates/tasks/index.plush.html`

Ampliar el `<p class="list-count">` para añadir el desglose después del total, reutilizando el patrón singular/plural ya usado:

```html
<p class="list-count">
  <%= if (query != "") { %>
    <%= len(tasks) %> <%= if (len(tasks) == 1) { %>resultado para<% } else { %>resultados para<% } %> «<%= query %>»
  <% } else { %>
    <%= len(tasks) %> <%= if (len(tasks) == 1) { %>tarea en tu lista<% } else { %>tareas en tu lista<% } %>
  <% } %>
  · <%= pendingCount %> <%= if (pendingCount == 1) { %>pendiente<% } else { %>pendientes<% } %>
  · <%= completedCount %> <%= if (completedCount == 1) { %>completada<% } else { %>completadas<% } %>
</p>
```

No se toca `edit.plush.html` ni ningún otro template: el desglose solo tiene sentido en la vista de lista.

## Archivos relevantes

| Ruta | Responsabilidad en este cambio |
|---|---|
| `actions/tasks.go` | Calcula y expone `pendingCount`/`completedCount` en `TasksIndex` y en la rama de error de `TasksCreate` |
| `actions/tasks_test.go` | Tests del desglose, con y sin búsqueda activa |
| `templates/tasks/index.plush.html` | Añade el desglose al contador de la cabecera |

## Implementación paso a paso

1. En `TasksIndex`, calcular `pendingCount`/`completedCount` sobre `tasks` tras aplicar el filtro de búsqueda, y exponerlos a la plantilla.
2. En la rama de error de `TasksCreate`, calcular y exponer las mismas dos variables sobre la `tasks` que ya recarga esa rama.
3. Actualizar `templates/tasks/index.plush.html` para mostrar el desglose en `<p class="list-count">`.
4. Ejecutar `gofmt -l .`, `go vet ./...`, `go build ./...`.
5. Ejecutar `GO_ENV=test go test -p 1 ./actions/...` con las migraciones de test ya aplicadas.

## Tests y validación

- `actions/tasks_test.go`:
  - Con la fixture `"tres tareas"` (2 pendientes, 1 completada, coincide con el ejemplo de la issue), `GET /` muestra "2 pendientes" y "1 completada".
  - Con una búsqueda activa (`?q=...`) que solo deje una tarea pendiente de las tres, el desglose refleja ese subconjunto filtrado (p. ej. "1 pendiente · 0 completadas"), no el total de las tres.
  - Sin tareas (`Test_TasksIndex_EmptyState` ya existente), el desglose no rompe el render; puede mostrar "0 pendientes · 0 completadas" o puede comprobarse solo que la página sigue devolviendo 200.
- Comandos: `gofmt -l .`, `go vet ./...`, `go build ./...`, `GO_ENV=test go test -p 1 ./actions/...` (no se toca el modelo, no hace falta `./models/...`).

## Riesgos y recuperación

- **Variable no expuesta en alguna rama de render**: si se olvida exponer `pendingCount`/`completedCount` en la rama de error de `TasksCreate` (paso 2), esa rama rompería con un 500 en vez de un 422 al reutilizar la misma plantilla. Mitigación: paso 2 explícito, igual que ya ocurrió con `query` al añadir la búsqueda.
- **Ejecución desatendida (modo issue de `/adw`)**: este plan no incluye una comprobación manual en navegador porque esta ejecución es automática; la validación se limita a los tests y comandos listados arriba. Si alguna comprobación visual (alineación, espaciado del `·`) resulta necesaria, queda pendiente de una revisión humana posterior sobre el Pull Request.
