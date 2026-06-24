-- Units of Measure: enforce case-insensitive name uniqueness ("Yards" and
-- "yards" are the same unit), per context/features/units-of-measure.md §10.
-- The plain UNIQUE constraint from 001 is case-sensitive, so back it with a
-- functional unique index instead.
CREATE UNIQUE INDEX uniq_units_of_measure_name_lower ON units_of_measure (LOWER(name));

-- Seed the starter units the business already uses, unblocking Products and
-- Receiving immediately (context/features/units-of-measure.md §5, "Seeding").
INSERT INTO units_of_measure (name) VALUES
    ('Yards'),
    ('Meters'),
    ('Pieces'),
    ('Rolls')
ON CONFLICT DO NOTHING;
