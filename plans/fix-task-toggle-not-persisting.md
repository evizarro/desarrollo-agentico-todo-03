# Feature: corregir que completar una tarea no persiste al recargar

## Encargo y criterios de aceptación

Encargo (literal): "corregir un bug que hay que no permite hacer check en las tareas, al recargar se desmarca".

Criterios de aceptación:

- Al pulsar el botón de completar una tarea, la tarea queda marcada como completada de verdad en el servidor (no solo visualmente).
- Al recargar la página (`GET /`), la tarea sigue apareciendo como completada.
- Lo mismo a la inversa: reabrir una tarea completada persiste tras recargar.
- El comportamiento visual inmediato (actualización optimista del DOM sin esperar respuesta) se conserva; no se pide eliminarlo, solo que deje de ser puramente cosmético.
- Si la petición al servidor fallara (red, sesión caducada, etc.), la interfaz no debe quedar mostrando un estado que no es real: debe revertir el cambio optimista.

## Comportamiento actual y propuesta

**Actual**: `templates/tasks/index.plush.html` renderiza, por cada tarea, un formulario `<form action="/tasks/<id>/toggle" method="POST" class="task-toggle" data-task-toggle>` con un campo oculto `_method=PUT` y el `authenticity_token`. La ruta real en `actions/app.go` es `app.PUT("/tasks/{task_id}/toggle", TasksToggle)` — no existe ninguna ruta `GET` para esa URL.

`public/assets/app.js` (líneas 1-35) intercepta el `submit` de ese formulario (`event.preventDefault()`), actualiza el DOM de forma optimista (clase `task-completed`, texto del botón, `aria-pressed`, nodo `.task-status`) y al final (línea 34) hace:

```js
fetch(form.action);
```

Esto es la causa raíz: `fetch` sin segundo argumento envía un `GET`, no el `PUT` que requiere la ruta. Como no hay ninguna ruta `GET` que la atienda, el servidor responde con un error (no hay handler para ese método en esa ruta) y, como el código no comprueba la respuesta del `fetch` en absoluto, el fallo pasa completamente desapercibido: el DOM ya se actualizó de forma optimista, así que visualmente parece que funcionó, pero el backend nunca ejecuta `TasksToggle` y `Completed` nunca cambia en base de datos. Al recargar (`GET /`), `TasksIndex` lee el valor real de la base de datos y la tarea vuelve a aparecer sin completar.

El backend en sí no tiene ningún problema: `actions/tasks.go#TasksToggle` (líneas 106-120) y su test `Test_TasksToggle_CompletesAndReopens` (en `actions/tasks_test.go`) ya demuestran que, si la petición `PUT` llega correctamente, el toggle persiste bien. El bug está exclusivamente en el JavaScript del cliente.

Confirmación adicional sobre el middleware CSRF (`github.com/gobuffalo/middleware@v1.0.0/csrf/csrf.go`): los métodos "seguros" exentos de comprobación son `GET, HEAD, OPTIONS, TRACE` (línea ~30); `PUT` no lo es, así que cualquier petición `PUT` real necesita el token CSRF, que el middleware acepta de dos formas (función `requestCSRFToken`, líneas 223-230): cabecera `X-CSRF-Token`, o valor de formulario `authenticity_token` en una petición con cuerpo `POST`/multipart. Como un `fetch` con `method: "PUT"` sin cuerpo no es un "POST form value", la vía viable para una petición `fetch` es la cabecera `X-CSRF-Token`, usando el valor que ya trae el campo oculto `authenticity_token` del propio formulario.

**Propuesto**: corregir únicamente `public/assets/app.js` para que la petición de confirmación se haga con el método y la cabecera CSRF correctos, comprobando la respuesta y revirtiendo el cambio optimista del DOM si la petición falla. No se toca ningún archivo del backend (el bug no está ahí) ni nada relacionado con la funcionalidad de prioridad añadida recientemente.

## Alcance

**Incluido**:
- Corregir la llamada `fetch` en `public/assets/app.js` para el toggle de completado: método `PUT`, cabecera `X-CSRF-Token` con el valor del campo `authenticity_token` del formulario, y comprobación de la respuesta.
- Si la petición falla (red o respuesta no "ok"), revertir el cambio optimista ya aplicado al DOM (clase `task-completed`, texto del botón, atributos `aria-*`, nodo `.task-status`) para que la interfaz no mienta sobre el estado real.
- Subir la versión del asset JS (`?v=2` → `?v=3` en `templates/application.plush.html`) para evitar que navegadores con caché sigan sirviendo el archivo roto.

**Excluido**:
- Cualquier cambio en `actions/tasks.go`, en las rutas de `actions/app.go`, en el modelo o en las migraciones: el backend del toggle ya funciona correctamente.
- Cualquier cambio relacionado con la funcionalidad de prioridad (`models/task.go`, migraciones de `priority`, badges, selects de prioridad): queda fuera de este encargo y no debe tocarse.
- Añadir un framework de tests de JavaScript: el repo no tiene ninguno (no hay `package.json` ni runner de JS); la validación de este fix es manual en navegador, además de confirmar que la suite de tests Go existente (que ya cubre el toggle a nivel de servidor) sigue en verde.
- Cambiar el comportamiento de otros formularios con `fetch`/JS (el de borrado usa `data-confirm` pero se envía como formulario normal, sin `fetch`; no está afectado por este bug).

## Diseño por capas

### Cliente — `public/assets/app.js`

Sustituir la línea `fetch(form.action);` por una petición que:

1. Lea el token CSRF del propio formulario: `var token = form.querySelector('[name="authenticity_token"]').value;` (el campo ya existe en el HTML, no hace falta tocar el template).
2. Guarde el estado anterior al cambio optimista (para poder revertir): el valor de `completed` antes de aplicar el `toggle` de la clase, y/o el nodo `status` eliminado si corresponde.
3. Envíe la petición con el método real y la cabecera CSRF:
   ```js
   fetch(form.action, {
     method: "PUT",
     credentials: "same-origin",
     headers: { "X-CSRF-Token": token }
   })
     .then(function (response) {
       if (!response.ok) {
         throw new Error("toggle failed");
       }
     })
     .catch(function () {
       // revertir el cambio optimista aplicado antes del fetch
     });
   ```
4. La reversión en el `catch` debe deshacer exactamente lo que se aplicó de forma optimista: alternar de nuevo `task-completed`, restaurar el texto/atributos del botón al estado previo, y volver a añadir o quitar el nodo `.task-status` según corresponda. La forma más simple y menos propensa a errores es extraer la aplicación del estado optimista a una función `applyState(completed)` y llamarla tanto para aplicar el cambio como, con el valor contrario, para revertirlo en el `catch`.
5. `credentials: "same-origin"` se incluye explícitamente para no depender del valor por defecto de `fetch` en cada navegador respecto al envío de cookies de sesión (necesarias porque el middleware CSRF compara el token recibido contra el guardado en la sesión).

No se necesita tocar el campo oculto `_method` ni replicar el truco de "method override" que usan los formularios HTML normales: al ser una petición `fetch`, se puede indicar `method: "PUT"` directamente, que es el método real que espera la ruta.

### Renderizado — `templates/application.plush.html`

Subir `app.js?v=2` a `app.js?v=3` para forzar que los navegadores con el script cacheado descarguen la versión corregida.

## Archivos relevantes

| Ruta | Responsabilidad en este cambio |
|---|---|
| `public/assets/app.js` | Corrige el método, la cabecera CSRF y añade manejo de fallo del toggle |
| `templates/application.plush.html` | Bump de `?v=` del JS |

## Implementación paso a paso

1. En `public/assets/app.js`, extraer la aplicación del estado optimista (clase `task-completed`, texto/atributos del botón, nodo `.task-status`) a una función reutilizable que reciba el booleano `completed` deseado.
2. Llamar a esa función con el nuevo estado (como ya hace el código actual) antes de lanzar la petición.
3. Sustituir `fetch(form.action);` por la petición `PUT` con cabecera `X-CSRF-Token` (tomada del campo `authenticity_token` del propio formulario) y `credentials: "same-origin"`.
4. En caso de respuesta no exitosa o error de red, volver a llamar a la función del paso 1 con el estado contrario para revertir el cambio optimista.
5. Subir `?v=2` → `?v=3` en el enlace a `app.js` dentro de `templates/application.plush.html`.
6. Reconstruir y levantar la app (`docker compose up -d --build`) y comprobar manualmente en el navegador: completar una tarea, recargar y confirmar que sigue completada; reabrirla, recargar y confirmar que sigue pendiente.
7. Ejecutar la suite de tests Go existente para confirmar que no hay regresión en el backend (el bug era solo de cliente, así que debe seguir en verde sin cambios).

## Tests y validación

- No se añade ningún test automatizado nuevo: el bug está en JavaScript sin framework de pruebas en este repo, y el comportamiento del servidor (que ya es correcto) está cubierto por el test existente `Test_TasksToggle_CompletesAndReopens` en `actions/tasks_test.go`, que no necesita cambios.
- Validación manual obligatoria en navegador (no sustituible por tests): completar y reabrir una tarea, recargar la página en cada caso y confirmar que el estado persiste. Opcionalmente, simular un fallo de red (p. ej. DevTools → Network → Offline) para comprobar que el cambio optimista se revierte en vez de quedar en un estado falso.
- Comandos de validación del backend (sin cambios esperados, solo para confirmar que no hay regresión):
  - `go vet ./...`
  - `go build ./...`
  - `GO_ENV=test go test -p 1 ./actions/... ./models/...`

## Riesgos y recuperación

- **Caché del navegador**: si se olvida subir `?v=` del script, un navegador con el JS cacheado seguiría ejecutando la versión rota. Mitigación: paso 5 explícito.
- **Nombre del campo CSRF**: si el campo oculto del formulario no se llama exactamente `authenticity_token` en el DOM (coincide con `fieldName` del middleware), la cabecera `X-CSRF-Token` llevaría un valor vacío y el servidor devolvería 403. Ya verificado contra `templates/tasks/index.plush.html`, que usa `name="authenticity_token"`; si en el futuro cambia ese nombre, este fix se rompe en silencio salvo que se revise a la vez.
- **Revertir el estado optimista de forma incompleta**: si la función de reversión no replica exactamente la lógica de aplicación (por ejemplo, olvida restaurar el `aria-label` con el título de la tarea), el botón podría quedar con atributos de accesibilidad inconsistentes tras un fallo de red. Mitigación: usar la misma función para aplicar y revertir (paso 1), no dos implementaciones distintas.
