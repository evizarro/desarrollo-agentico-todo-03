// Pide confirmación antes de enviar formularios marcados con data-confirm.
document.addEventListener("submit", function (event) {
  var form = event.target;
  if (form.dataset.confirm && !window.confirm(form.dataset.confirm)) {
    event.preventDefault();
  }

  if (!form.matches("[data-task-toggle]")) {
    return;
  }

  event.preventDefault();

  var task = form.closest(".task");
  var button = form.querySelector("button");

  function applyState(completed) {
    task.classList.toggle("task-completed", completed);

    var label = completed ? "Reabrir" : "Completar";
    button.querySelector(".toggle-label").textContent = label;
    button.setAttribute("aria-pressed", String(completed));
    button.setAttribute("aria-label", label + " tarea: " + task.querySelector(".task-title").textContent);
    button.setAttribute("title", label + " tarea");

    var status = task.querySelector(".task-status");
    if (completed && !status) {
      status = document.createElement("span");
      status.className = "task-status";
      status.textContent = "Completada";
      task.insertBefore(status, task.querySelector(".task-actions"));
    } else if (!completed && status) {
      status.remove();
    }
  }

  var wasCompleted = task.classList.contains("task-completed");
  applyState(!wasCompleted);

  var token = form.querySelector('[name="authenticity_token"]').value;

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
      applyState(wasCompleted);
    });
});
