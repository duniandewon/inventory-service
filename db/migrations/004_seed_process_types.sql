INSERT INTO process_types (name)
VALUES ('Cutting'), ('Sewing'), ('Printing')
ON CONFLICT (name) DO NOTHING;
