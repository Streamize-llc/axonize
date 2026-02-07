-- Add gpu_vendors column to spans table (was parsed by server but not persisted).
ALTER TABLE spans ADD COLUMN IF NOT EXISTS gpu_vendors Array(String) AFTER gpu_models;
