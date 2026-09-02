-- +goose Up

INSERT INTO exercises (name, category) VALUES
    ('Back Squat', 'legs'),
    ('Front Squat', 'legs'),
    ('Goblet Squat', 'legs'),
    ('Bulgarian Split Squat', 'legs'),
    ('Leg Press', 'legs'),
    ('Walking Lunge', 'legs'),
    ('Romanian Deadlift', 'legs'),
    ('Hip Thrust', 'legs'),
    ('Standing Calf Raise', 'legs'),
    ('Conventional Deadlift', 'full_body'),
    ('Bench Press', 'chest'),
    ('Incline Bench Press', 'chest'),
    ('Dumbbell Bench Press', 'chest'),
    ('Push-Up', 'chest'),
    ('Dip', 'chest'),
    ('Overhead Press', 'shoulders'),
    ('Dumbbell Shoulder Press', 'shoulders'),
    ('Lateral Raise', 'shoulders'),
    ('Face Pull', 'shoulders'),
    ('Barbell Row', 'back'),
    ('Dumbbell Row', 'back'),
    ('Lat Pulldown', 'back'),
    ('Seated Cable Row', 'back'),
    ('Pull-Up', 'back'),
    ('Chin-Up', 'back'),
    ('Barbell Bicep Curl', 'arms'),
    ('Hammer Curl', 'arms'),
    ('Tricep Pushdown', 'arms'),
    ('Plank', 'core'),
    ('Hanging Leg Raise', 'core')
ON CONFLICT (name) DO NOTHING;

-- +goose Down

-- Forward-only catalog: rolling back this version does not DELETE exercises.
-- A full reset is schema down (00001), which drops the table.
