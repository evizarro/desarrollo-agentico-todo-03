# Feature: Mostrar hace cuánto se creó cada tarea

## Encargo y criterios de aceptación

Issue #5: junto al título de cada tarea en la lista, mostrar desde cuándo existe en
lenguaje natural (p. ej. "hace 2 días", "hoy", "hace 3 semanas"), usando la fecha de
creación (`created_at`) que el modelo `Task` ya persiste. No requiere ningún dato
nuevo en base de datos.

Criterios de aceptación (explícitos en la issue):

- En `GET /` (y en los resultados de búsqueda, que reutilizan la misma lista), cada
  tarea muestra junto a su título una expresión en español de cuánto hace que se
  creó, derivada de `created_at`.
- No se añade ninguna columna ni migración nueva: el dato ya existe.

Inferencias (no pedidas explícitamente, pero necesarias para que "en lenguaje
natural" sea coherente en todos los casos, no solo en los tres ejemplos dados):

- Se definen umbrales completos (segundos/minutos recientes, "hoy", "ayer", días,
  semanas, meses, años) en vez de cubrir solo los tres ejemplos de la issue.
- Semanas, meses y años se calculan por aproximación de días (7, 30, 365), no por
  calendario exacto (meses de distinta duración, años bisiestos). Se documenta como
  simplificación aceptada en "Riesgos y recuperación".
- La comparación de "hoy"/"ayer" se hace por fecha de calendario en UTC (no por
  huso horario local), ya que `created_at` se persiste en UTC vía Postgres/pop.
- Pluralización correcta en español ("1 día" vs "2 días", "1 semana" vs "2
  semanas", etc.).

## Comportamiento actual y propuesta

**Actual**: `templates/tasks/index.plush.html` renderiza cada tarea con título,
prioridad y estado de completado (`actions/tasks.go:TasksIndex`), pero no muestra
nada relativo a `t.CreatedAt`.

**Propuesto**: añadir una función pura en Go que traduzca una fecha a una cadena
relativa en español, registrarla como helper de plantilla Plush, y usarla en
`index.plush.html` junto al título de cada tarea.

## Alcance

**Incluido**:
- Función pura `models.TimeAgo(from, to time.Time) string` con sus tests unitarios.
- Registro de un helper Plush `timeAgo` en `actions/render.go` que la invoque con
  `time.Now()`.
- Uso de `timeAgo(t.CreatedAt)` en `templates/tasks/index.plush.html`, junto al
  título de cada tarea.
- Estilo mínimo en `public/assets/app.css` para que el texto no rompa el layout
  existente (reutilizando el patrón visual "texto discreto" ya usado en el resto
  de la lista).
- Tests de integración en `actions/tasks_test.go` que verifiquen que la cadena
  aparece en la respuesta HTML para un par de casos representativos.

**Excluido**:
- Cualquier columna o migración nueva (la issue lo descarta explícitamente).
- Actualización en vivo del texto vía JavaScript (se recalcula solo al recargar
  la página, igual que el resto de la UI, que ya es de recarga completa).
- Tooltip con la fecha/hora exacta al pasar el ratón (mejora razonable pero no
  pedida; se puede añadir después sin impacto en este plan).
- Internacionalización a otros idiomas (la app ya está íntegramente en español).
- Cambios en `templates/tasks/edit.plush.html` (la issue habla de "la lista").

## Diseño por capas

- **Modelo (lógica de dominio pura)**: `models.TimeAgo`, sin dependencia de la
  base de datos ni de Buffalo. Testeable con `go test` normal, sin Postgres.
- **Presentación (helper Plush)**: `actions/render.go` expone `timeAgo` a las
  plantillas, delegando en `models.TimeAgo` con `time.Now()` como referencia
  actual.
- **Vista**: `templates/tasks/index.plush.html` invoca `timeAgo(t.CreatedAt)`.
- **Estilos**: `public/assets/app.css` añade una regla `.task-age` discreta.

No se tocan rutas, controladores más allá de lo ya existente, ni el modelo
`Task` (no se añade ningún campo).

## Archivos relevantes

- `models/time_ago.go` (nuevo): función pura `TimeAgo` y sus helpers internos de
  pluralización y cálculo de días de calendario.
- `models/time_ago_test.go` (nuevo): tests de tabla cubriendo todos los umbrales.
- `actions/render.go`: registra el helper `timeAgo` en el `render.Helpers` ya
  existente.
- `templates/tasks/index.plush.html`: añade el `<span>` con el texto relativo
  junto al título de cada tarea.
- `public/assets/app.css`: añade la regla `.task-age`.
- `actions/tasks_test.go`: añade tests de integración que comprueben el texto
  renderizado en la lista.

## Implementación paso a paso

1. **Crear `models/time_ago.go`**:

   ```go
   package models

   import (
       "fmt"
       "time"
   )

   // TimeAgo describe, en español y en lenguaje natural, cuánto tiempo ha
   // pasado entre from y to (to es normalmente time.Now()).
   func TimeAgo(from, to time.Time) string {
       elapsed := to.Sub(from)
       if elapsed < time.Minute {
           return "justo ahora"
       }
       if elapsed < time.Hour {
           minutes := int(elapsed.Minutes())
           return fmt.Sprintf("hace %d %s", minutes, pluralize(minutes, "minuto", "minutos"))
       }

       days := daysBetween(from, to)
       switch {
       case days == 0:
           return "hoy"
       case days == 1:
           return "ayer"
       case days < 7:
           return fmt.Sprintf("hace %d %s", days, pluralize(days, "día", "días"))
       case days < 30:
           weeks := days / 7
           return fmt.Sprintf("hace %d %s", weeks, pluralize(weeks, "semana", "semanas"))
       case days < 365:
           months := days / 30
           return fmt.Sprintf("hace %d %s", months, pluralize(months, "mes", "meses"))
       default:
           years := days / 365
           return fmt.Sprintf("hace %d %s", years, pluralize(years, "año", "años"))
       }
   }

   // daysBetween calcula la diferencia de días de calendario (en UTC) entre
   // from y to, independientemente de la hora del día.
   func daysBetween(from, to time.Time) int {
       from = from.UTC()
       to = to.UTC()
       fromDate := time.Date(from.Year(), from.Month(), from.Day(), 0, 0, 0, 0, time.UTC)
       toDate := time.Date(to.Year(), to.Month(), to.Day(), 0, 0, 0, 0, time.UTC)
       return int(toDate.Sub(fromDate).Hours() / 24)
   }

   func pluralize(n int, singular, plural string) string {
       if n == 1 {
           return singular
       }
       return plural
   }
   ```

   Nota: si `elapsed` es negativo (reloj desincronizado o `created_at` en el
   futuro), cae en la rama `elapsed < time.Minute` y devuelve "justo ahora", sin
   mostrar valores negativos.

2. **Registrar el helper en `actions/render.go`**: añadir los imports `"time"` y
   `"todo/models"`, y en el mapa `Helpers` añadir:

   ```go
   Helpers: render.Helpers{
       "timeAgo": func(t time.Time) string {
           return models.TimeAgo(t, time.Now())
       },
   },
   ```

3. **Usar el helper en `templates/tasks/index.plush.html`**: dentro del `<span
   class="task-content">`, justo después de `task-title` y antes de
   `task-priority`, añadir:

   ```html
   <span class="task-title"><%= t.Title %></span>
   <span class="task-age"><%= timeAgo(t.CreatedAt) %></span>
   <span class="task-priority ...">
   ```

4. **Añadir estilo en `public/assets/app.css`**, junto a las reglas de
   `.task-title`/`.task-priority` (línea ~160), reutilizando `var(--color-muted)`
   ya usado en el resto de la hoja:

   ```css
   .task-age {
     flex-shrink: 0;
     font-size: 0.8rem;
     color: var(--color-muted);
     white-space: nowrap;
   }
   ```

5. **Tests de integración** en `actions/tasks_test.go`: añadir un test que cree
   una tarea, le fuerce `created_at` a una fecha conocida mediante SQL directo
   (porque `pop` sobrescribe `created_at`/`updated_at` al crear), y compruebe el
   texto en la respuesta:

   ```go
   func (as *ActionSuite) Test_TasksIndex_ShowsRelativeAge() {
       task := &models.Task{Title: "Tarea de hace dos días"}
       verrs, err := as.DB.ValidateAndCreate(task)
       as.NoError(err)
       as.False(verrs.HasAny())

       twoDaysAgo := time.Now().UTC().AddDate(0, 0, -2)
       as.NoError(as.DB.RawQuery(
           "update tasks set created_at = ? where id = ?", twoDaysAgo, task.ID,
       ).Exec())

       res := as.HTML("/").Get()
       as.Equal(200, res.Code)
       as.Contains(res.Body.String(), "hace 2 días")
   }

   func (as *ActionSuite) Test_TasksIndex_ShowsTodayForRecentTask() {
       task := &models.Task{Title: "Tarea recién creada"}
       verrs, err := as.DB.ValidateAndCreate(task)
       as.NoError(err)
       as.False(verrs.HasAny())

       res := as.HTML("/").Get()
       as.Equal(200, res.Code)
       body := res.Body.String()
       as.True(strings.Contains(body, "justo ahora") || strings.Contains(body, "hoy"))
   }
   ```

   Añadir los imports `"time"` y, si no está ya, `"strings"` en
   `actions/tasks_test.go` (ya existe `"strings"` en el archivo).

## Tests y validación

Nuevos/actualizados:
- `models/time_ago_test.go` (nuevo): tabla de casos cubriendo "justo ahora"
  (0s y negativo), minutos (1 y N), "hoy" (mismo día, ≥1h), "ayer", días (2 a 6,
  singular/plural), semanas (1 y N), meses (1 y N), años (1 y N). No requiere
  Postgres: son funciones puras con `*testing.T` normal (no `ModelSuite`).
- `actions/tasks_test.go`: los dos tests nuevos de arriba, que sí requieren la
  base de datos de test ya configurada en el proyecto.

Comandos de validación:
- `gofmt -l .` (debe no listar archivos).
- `go vet ./...`
- `go build ./...`
- `go test ./models/... -run TestTimeAgo -v` y/o `go test -p 1 ./models/...` para
  confirmar que los tests puros pasan sin depender de que la suite con DB se
  ejecute primero.
- `go test -p 1 ./...` (suite completa; requiere PostgreSQL arrancado y las
  migraciones de test aplicadas, como ya exige el resto del proyecto).

## Riesgos y recuperación

- **Aproximación de semanas/meses/años por días fijos (7/30/365)**: puede
  desviarse ligeramente del calendario real (p. ej. un mes de 31 días se seguirá
  llamando "1 mes" durante su día 31). Aceptado como simplificación razonable
  para "lenguaje natural" aproximado; si se necesita precisión de calendario
  exacta, se puede sustituir `daysBetween`/los divisores por aritmética de
  `time.AddDate` en una iteración posterior, sin romper la firma pública de
  `TimeAgo`.
- **Zona horaria**: se asume que `created_at` se persiste en UTC (comportamiento
  por defecto de pop/Postgres en este proyecto). Si en el futuro se guarda en
  otro huso, `daysBetween` debe actualizarse a la vez.
- **Tests de integración dependen de `RawQuery` para forzar `created_at`**: si
  `pop` cambia su comportamiento de autopoblado de timestamps, el `UPDATE` SQL
  directo seguiría funcionando igualmente al operar sobre la columna en crudo.
- **Rollback**: el cambio es aditivo (una función nueva, un helper nuevo, una
  línea de plantilla y una regla CSS). Revertir es tan simple como deshacer el
  commit; no hay migraciones que revertir ni datos persistidos que limpiar.
