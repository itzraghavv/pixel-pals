-- +goose Up
CREATE TABLE space_area_tiles (
    space_id text NOT NULL REFERENCES spaces(id) ON DELETE CASCADE,
    x integer NOT NULL CHECK (x >= 0),
    y integer NOT NULL CHECK (y >= 0),
    area_id text NOT NULL CHECK (area_id <> ''),
    PRIMARY KEY (space_id, x, y)
);

-- +goose Down
DROP TABLE space_area_tiles;
