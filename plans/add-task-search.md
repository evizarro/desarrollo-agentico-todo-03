# Feature: buscar tareas por título

## Encargo y criterios de aceptación

Encargo (literal): "añadir un campo de búsqueda que filtre la lista de tareas por el texto del título".

Criterios de aceptación:

- En la página principal (`/`) hay un campo de texto para buscar tareas por título.
- Al buscar, la lista solo muestra las tareas cuyo título contiene el texto buscado (coincidencia parcial, no exacta).
- La búsqueda no distingue mayúsculas/minúsculas. *(Inferencia: no especificado; es el comportamiento esperado en cualquier buscador simple.)*
- Si la búsqueda no encuentra ninguna tarea, se indica claramente que no hay resultados para ese texto (distinto del mensaje de "no hay tareas todavía" cuando la lista está realmente vacía).
- Vaciar el campo de búsqueda (o no haber buscado nunca) muestra todas las tareas, en el mismo orden que hoy.
- Marcar/desmarcar o borrar una tarea mientras hay una búsqueda activa no debe perder el filtro: tras la acción, se vuelve a la misma búsqueda, no a la lista completa. *(Inferencia: si no se preserva, la búsqueda "se rompe" nada más usar cualquier acción sobre una tarea filtrada, lo que contradice el propósito de la función.)*

## Comportamiento actual y propuesta

**Actual**: `TasksIndex` (`actions/tasks.go`, líneas 19-31) carga siempre todas las tareas (`tx.Order(taskOrder).All(&tasks)`, sin ningún `Where`) y las renderiza en `templates/tasks/index.plush.html`. No existe ningún parámetro de consulta ni campo de búsqueda. `TasksToggle` y `TasksDestroy` redirigen siempre a `/` a secas tras completar su acción (líneas 131 y 148).

**Propuesto**: añadir un parámetro de consulta `q` a `GET /` que, cuando venga no vacío, filtre las tareas por título con `ILIKE '%q%'` (case-insensitive, coincidencia parcial) antes de aplicar el orden ya existente. Añadir un formulario de búsqueda (método `GET`, sin JavaScript, consistente con el resto de la app) en la cabecera de `index.plush.html`, y propagar el valor de `q` como parámetro en los redirects de `TasksToggle` y `TasksDestroy` para no perder el filtro al actuar sobre una tarea visible en los resultados.

No se toca nada de la funcionalidad de prioridad (orden `taskOrder`, badges, selects) ni del fix de persistencia del toggle: ambos quedan intactos, la búsqueda se añade por encima sin alterar su comportamiento.

## Alcance

**Incluido**:
- Campo de búsqueda en la cabecera de `/` que filtra por título (parcial, case-insensitive) vía `?q=`.
- Mensaje distinto para "sin tareas en absoluto" frente a "sin resultados para esta búsqueda".
- Preservar `?q=` en los redirects de completar/reabrir (`TasksToggle`) y borrar (`TasksDestroy`) una tarea, para no perder el filtro tras esas acciones.
- Un enlace o botón para limpiar la búsqueda y volver a ver todas las tareas.

**Excluido** (decisión explícita para mantener el cambio acotado):
- Búsqueda insensible a tildes/acentos (`ILIKE` en Postgres no normaliza acentos sin la extensión `unaccent`, que no está instalada; añadirla implicaría una migración nueva fuera del alcance de este encargo). Se documenta como limitación conocida, no se resuelve aquí.
- Buscar por otros campos (prioridad, estado completada/pendiente): solo título, como pide el encargo.
- Filtrado instantáneo por JavaScript sin recargar la página: se usa un formulario `GET` normal, consistente con el resto de la app (que no tiene ningún filtrado dinámico hoy); recargar al buscar es aceptable y más simple.
- Preservar `?q=` al crear una tarea (`TasksCreate`): tras crear, se vuelve a la lista completa sin filtro, para evitar la confusión de "creé una tarea y no la veo" si no coincide con la búsqueda activa.
- Paginación de resultados: no existe hoy para la lista completa: tampoco se añade aquí.

## Diseño por capas

### Rutas y acciones — `actions/tasks.go`

- `TasksIndex`: leer `q := strings.TrimSpace(c.Param("q"))`. Si `q != ""`, aplicar `tx.Where("title ILIKE ?", "%"+q+"%")` antes de `.Order(taskOrder).All(&tasks)`. Exponer `q` a la plantilla (`c.Set("query", q)`) para rellenar el campo de búsqueda y decidir qué mensaje de "vacío" mostrar.
- `TasksToggle`: tras el `tx.Update(task)`, redirigir a `/` incluyendo `q` si venía en la petición (`c.Param("q")`), p. ej. `c.Redirect(http.StatusSeeOther, "/?q="+url.QueryEscape(c.Param("q")))` cuando `q` no esté vacío, o `"/"` si está vacío (para no ensuciar la URL con `?q=` cuando no hace falta). El formulario de toggle necesita entonces un campo oculto `q` con el valor de la búsqueda activa para que `c.Param("q")` lo reciba en el `POST`.
- `TasksDestroy`: mismo tratamiento que `TasksToggle` para preservar `q` en el redirect.
- `TasksCreate`: sin cambios (sigue redirigiendo a `/` sin `q`, según lo excluido arriba).

### Modelo — sin cambios

No se toca `models/task.go`: la búsqueda se resuelve con una condición `Where` en la consulta existente, no requiere un método nuevo en el modelo.

### Renderizado — `templates/tasks/index.plush.html`

- Añadir, en la cabecera (`<header class="page-header">`, antes o junto al contador de tareas), un formulario de búsqueda:
  ```html
  <form action="/" method="GET" class="task-search" role="search">
    <label for="task-search" class="sr-only">Buscar tareas</label>
    <input id="task-search" type="search" name="q" value="<%= query %>" placeholder="Buscar por título…">
    <button type="submit" class="button">Buscar</button>
    <%= if (query != "") { %><a href="/" class="button">Limpiar</a><% } %>
  </form>
  ```
- Ajustar el contador de tareas (`<p class="list-count">`) para que, cuando `query != ""`, indique cuántos resultados hay para esa búsqueda (reutilizando `len(tasks)`, que ya refleja el filtro aplicado en `TasksIndex`).
- Ajustar el bloque de "lista vacía": si `len(tasks) == 0`, mostrar "No hay tareas todavía..." solo cuando `query == ""`; si `query != ""`, mostrar un mensaje del tipo "No se encontraron tareas para «<%= query %>»." con un enlace para limpiar la búsqueda.
- En cada formulario de `toggle` y en el de `destroy` dentro del `<li>` de cada tarea, añadir `<input type="hidden" name="q" value="<%= query %>">` para que el valor de búsqueda viaje de vuelta en el `POST` y la acción pueda preservarlo en el redirect.

## Archivos relevantes

| Ruta | Responsabilidad en este cambio |
|---|---|
| `actions/tasks.go` | `TasksIndex` filtra por `q`; `TasksToggle`/`TasksDestroy` preservan `q` en el redirect |
| `actions/tasks_test.go` | Tests del filtrado, de los mensajes de "sin resultados" y de la preservación de `q` |
| `templates/tasks/index.plush.html` | Formulario de búsqueda, mensajes de lista vacía, campos ocultos `q` en toggle/destroy |

## Implementación paso a paso

1. En `TasksIndex`, leer y recortar `q`, aplicar `tx.Where("title ILIKE ?", "%"+q+"%")` solo si `q != ""`, y exponer `query` a la plantilla.
2. En `TasksCreate` (rama de error 422, que también renderiza `index.plush.html`), exponer `query` como cadena vacía para que la plantilla no falle al no recibir esa variable (revisar si Plush requiere la clave presente o tolera su ausencia; si la tolera, se puede omitir, pero conviene ser explícito para no depender de ese detalle).
3. Añadir el formulario de búsqueda y los mensajes condicionales de "vacío" en `templates/tasks/index.plush.html`.
4. Añadir el campo oculto `q` a los formularios de `toggle` y `destroy` de cada tarea en la misma plantilla.
5. En `TasksToggle` y `TasksDestroy`, leer `c.Param("q")` y construir el redirect preservando ese valor cuando no esté vacío.
6. Reconstruir y levantar la app (`docker compose up -d --build`) y comprobar manualmente: buscar un título existente, confirmar que solo aparece esa tarea; buscar un texto sin coincidencias, confirmar el mensaje de "sin resultados"; limpiar la búsqueda y confirmar que vuelven todas; con una búsqueda activa, completar y borrar una tarea de los resultados y confirmar que el filtro se mantiene tras la acción.
7. Ejecutar la suite de tests Go (`gofmt`, `go vet`, `go build`, `go test`).

## Tests y validación

- `actions/tasks_test.go`:
  - `TasksIndex` con `?q=<texto>` que coincide con una tarea: solo esa tarea aparece en el HTML de respuesta, las demás no.
  - `TasksIndex` con `?q=<texto>` que no coincide con ninguna: aparece el mensaje de "sin resultados", no el de "no hay tareas todavía".
  - `TasksIndex` sin `q` (o `q` vacío) sigue mostrando todas las tareas, igual que hoy.
  - La búsqueda es case-insensitive: buscar en minúsculas encuentra un título con mayúsculas.
  - `TasksToggle` llamado con `q` en el formulario redirige a `/?q=<mismo texto>`, no a `/` a secas.
  - `TasksDestroy` llamado con `q` en el formulario redirige a `/?q=<mismo texto>`.
- Comandos de validación:
  - `gofmt -l .`
  - `go vet ./...`
  - `go build ./...`
  - `GO_ENV=test go test -p 1 ./actions/...`

## Riesgos y recuperación

- **Acentos**: una búsqueda de "cafe" no encontrará "café" (ver Alcance → Excluido). Si resulta confuso en el uso real, requeriría la extensión `unaccent` de Postgres, fuera de este encargo.
- **`query` no definida en la rama de error de `TasksCreate`**: si Plush exige que toda variable usada en la plantilla esté presente en el contexto, no exponer `query` en esa rama rompería el render con un 500 en vez de un 422. Mitigación: paso 2 explícito, verificar al implementar si hace falta o no.
- **Caracteres especiales en `q`**: si el texto buscado contiene comillas o símbolos de patrón `ILIKE` (`%`, `_`), Postgres los tratará como comodines; como el placeholder sigue siendo parametrizado (`?`), no hay riesgo de inyección SQL, solo de que una búsqueda con `%` o `_` se comporte de forma algo distinta a lo esperado por el usuario. No se escapan esos caracteres en este plan por mantenerlo simple; si resulta molesto en el uso real, se podría tratar como una mejora posterior.
- **Olvidar el campo oculto `q` en algún formulario de tarea**: si se omite en `toggle` o `destroy`, esa acción concreta perdería el filtro al redirigir, aunque el resto de la búsqueda siga funcionando. Mitigación: paso 4 cubre explícitamente ambos formularios.
