-- Dados de demonstração (tenant "demo"), só para o ambiente local.
SELECT telemetry.create_tenant_role('demo', 'dev-only-change-me');

INSERT INTO garden.garden (tenant_id, id, name) VALUES ('demo', 'horta-1', 'Horta do quintal');
INSERT INTO garden.bed (tenant_id, garden_id, id, name, crop, irrigation) VALUES
    ('demo', 'horta-1', 'canteiro-1', 'Canteiro de alface', 'alface', 'drip'),
    ('demo', 'horta-1', 'canteiro-2', 'Canteiro de tomate', 'tomate', 'sprinkler');
INSERT INTO garden.node (tenant_id, garden_id, id, kind, bed_id) VALUES
    ('demo', 'horta-1', 'n1', 'sensor', 'canteiro-1'),
    ('demo', 'horta-1', 'n2', 'sensor', 'canteiro-2'),
    ('demo', 'horta-1', 'n3', 'actuator', 'canteiro-1');
INSERT INTO garden.node_state (tenant_id, garden_id, node_id) VALUES ('demo', 'horta-1', 'n3');
