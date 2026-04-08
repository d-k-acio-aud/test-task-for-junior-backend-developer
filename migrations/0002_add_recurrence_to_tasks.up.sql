ALTER TABLE tasks
	ADD COLUMN IF NOT EXISTS recurrence_type TEXT,
	ADD COLUMN IF NOT EXISTS recurrence_every_n_days INTEGER,
	ADD COLUMN IF NOT EXISTS recurrence_day_of_month INTEGER,
	ADD COLUMN IF NOT EXISTS recurrence_specific_dates DATE[],
	ADD COLUMN IF NOT EXISTS recurrence_parity TEXT,
	ADD COLUMN IF NOT EXISTS recurrence_start_date DATE,
	ADD COLUMN IF NOT EXISTS scheduled_for DATE,
	ADD COLUMN IF NOT EXISTS template_task_id BIGINT REFERENCES tasks(id) ON DELETE SET NULL;

CREATE UNIQUE INDEX IF NOT EXISTS ux_tasks_template_scheduled_for
	ON tasks (template_task_id, scheduled_for)
	WHERE template_task_id IS NOT NULL;
