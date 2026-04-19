-- +goose Up
INSERT INTO role_module_permissions (role, module, can_view, can_edit) VALUES
    ('operator', 'reports', 1, 0),
    ('viewer',   'reports', 1, 0);

-- +goose Down
DELETE FROM role_module_permissions WHERE module = 'reports';
