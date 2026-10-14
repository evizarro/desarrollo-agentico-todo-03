package actions

import (
	"net/http"
	"net/url"
	"strings"

	"todo/models"

	"github.com/gobuffalo/buffalo"
	"github.com/gobuffalo/pop/v6"
	"github.com/gobuffalo/validate/v3"
)

// taskOrder ordena las tareas por prioridad (alta, media, baja) y, dentro de
// cada nivel, por fecha de creación ascendente.
const taskOrder = "CASE priority WHEN 'high' THEN 0 WHEN 'medium' THEN 1 WHEN 'low' THEN 2 ELSE 3 END, created_at asc, id asc"

// Valores permitidos para el filtro de estado de la lista de tareas.
const (
	StatusAll       = "all"
	StatusPending   = "pending"
	StatusCompleted = "completed"
)

// normalizeStatus degrada cualquier valor desconocido o vacío a StatusAll.
func normalizeStatus(raw string) string {
	switch raw {
	case StatusPending, StatusCompleted:
		return raw
	default:
		return StatusAll
	}
}

// taskListURL construye la URL de "/" con los parámetros de estado y
// búsqueda que no sean el valor por defecto.
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

// setStatusTabs expone al contexto el estado activo y las URLs de las tres
// pestañas de filtro, para que la plantilla no construya query strings.
//
// La clave de contexto no puede llamarse "status": buffalo.DefaultContext.Render
// sobrescribe siempre esa clave con el código de estado HTTP de la respuesta
// (ver data["status"] = status en default_context.go), pisando cualquier valor
// que se le asigne antes con c.Set.
func setStatusTabs(c buffalo.Context, status, query string) {
	c.Set("taskStatus", status)
	c.Set("statusAllURL", taskListURL(StatusAll, query))
	c.Set("statusPendingURL", taskListURL(StatusPending, query))
	c.Set("statusCompletedURL", taskListURL(StatusCompleted, query))
}

// TasksIndex muestra la lista de tareas y el formulario de creación.
func TasksIndex(c buffalo.Context) error {
	tx := c.Value("tx").(*pop.Connection)

	query := strings.TrimSpace(c.Param("q"))
	status := normalizeStatus(c.Param("status"))

	q := tx.Order(taskOrder)
	if query != "" {
		q = q.Where("title ILIKE ?", "%"+query+"%")
	}
	switch status {
	case StatusPending:
		q = q.Where("completed = ?", false)
	case StatusCompleted:
		q = q.Where("completed = ?", true)
	}

	tasks := models.Tasks{}
	if err := q.All(&tasks); err != nil {
		return err
	}

	c.Set("tasks", tasks)
	c.Set("query", query)
	setStatusTabs(c, status, query)
	c.Set("task", &models.Task{})
	c.Set("errors", validate.NewErrors())
	return c.Render(http.StatusOK, r.HTML("tasks/index.plush.html"))
}

// TasksCreate crea una tarea nueva a partir del formulario.
func TasksCreate(c buffalo.Context) error {
	tx := c.Value("tx").(*pop.Connection)

	task := &models.Task{}
	if err := c.Bind(task); err != nil {
		return err
	}
	task.Title = strings.TrimSpace(task.Title)
	if task.Priority == "" {
		task.Priority = models.PriorityMedium
	}

	verrs, err := tx.ValidateAndCreate(task)
	if err != nil {
		return err
	}

	if verrs.HasAny() {
		tasks := models.Tasks{}
		if err := tx.Order(taskOrder).All(&tasks); err != nil {
			return err
		}

		c.Set("tasks", tasks)
		c.Set("query", "")
		setStatusTabs(c, StatusAll, "")
		c.Set("task", task)
		c.Set("errors", verrs)
		return c.Render(http.StatusUnprocessableEntity, r.HTML("tasks/index.plush.html"))
	}

	c.Flash().Add("success", "Tarea creada.")
	return c.Redirect(http.StatusSeeOther, "/")
}

// TasksEdit muestra el formulario de edición de una tarea existente.
func TasksEdit(c buffalo.Context) error {
	tx := c.Value("tx").(*pop.Connection)

	task := &models.Task{}
	if err := tx.Find(task, c.Param("task_id")); err != nil {
		return c.Error(http.StatusNotFound, err)
	}

	c.Set("task", task)
	c.Set("errors", validate.NewErrors())
	return c.Render(http.StatusOK, r.HTML("tasks/edit.plush.html"))
}

// TasksUpdate actualiza el título y la prioridad, conservando los demás datos de la tarea.
func TasksUpdate(c buffalo.Context) error {
	tx := c.Value("tx").(*pop.Connection)

	task := &models.Task{}
	if err := tx.Find(task, c.Param("task_id")); err != nil {
		return c.Error(http.StatusNotFound, err)
	}

	input := struct {
		Title    string `form:"title" json:"title"`
		Priority string `form:"priority" json:"priority"`
	}{}
	if err := c.Bind(&input); err != nil {
		return err
	}
	task.Title = strings.TrimSpace(input.Title)
	if input.Priority == "" {
		input.Priority = models.PriorityMedium
	}
	task.Priority = input.Priority

	verrs, err := tx.ValidateAndUpdate(task)
	if err != nil {
		return err
	}
	if verrs.HasAny() {
		c.Set("task", task)
		c.Set("errors", verrs)
		return c.Render(http.StatusUnprocessableEntity, r.HTML("tasks/edit.plush.html"))
	}

	c.Flash().Add("success", "Tarea actualizada.")
	return c.Redirect(http.StatusSeeOther, "/")
}

// TasksToggle alterna una tarea entre completada y pendiente.
func TasksToggle(c buffalo.Context) error {
	tx := c.Value("tx").(*pop.Connection)

	task := &models.Task{}
	if err := tx.Find(task, c.Param("task_id")); err != nil {
		return c.Error(http.StatusNotFound, err)
	}

	task.Completed = !task.Completed
	if err := tx.Update(task); err != nil {
		return err
	}

	return c.Redirect(http.StatusSeeOther, redirectToList(c))
}

// TasksDestroy elimina una tarea.
func TasksDestroy(c buffalo.Context) error {
	tx := c.Value("tx").(*pop.Connection)

	task := &models.Task{}
	if err := tx.Find(task, c.Param("task_id")); err != nil {
		return c.Error(http.StatusNotFound, err)
	}

	if err := tx.Destroy(task); err != nil {
		return err
	}

	c.Flash().Add("success", "Tarea eliminada.")
	return c.Redirect(http.StatusSeeOther, redirectToList(c))
}

// redirectToList devuelve "/" preservando "q" y "status" si venían en la
// petición, para no perder una búsqueda o un filtro activo tras completar o
// borrar una tarea.
func redirectToList(c buffalo.Context) string {
	query := strings.TrimSpace(c.Param("q"))
	status := normalizeStatus(c.Param("status"))
	return taskListURL(status, query)
}
