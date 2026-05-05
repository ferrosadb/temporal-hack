-- Temporal needs its own logical databases. Created once at first
-- lab bring-up; Temporal then runs its own schema migrator on top.
CREATE DATABASE temporal;
CREATE DATABASE temporal_visibility;
