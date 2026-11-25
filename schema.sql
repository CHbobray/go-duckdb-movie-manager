CREATE TABLE movies (movie_id BIGINT PRIMARY KEY, name VARCHAR NOT NULL, year INTEGER NOT NULL, rank DOUBLE NOT NULL);
CREATE TABLE actors (actor_id BIGINT PRIMARY KEY, first_name VARCHAR NOT NULL, last_name VARCHAR NOT NULL, gender VARCHAR NOT NULL);
CREATE TABLE genres (movie_id BIGINT REFERENCES movies(movie_id), genre VARCHAR, PRIMARY KEY(movie_id, genre));
CREATE TABLE roles (role_id BIGINT PRIMARY KEY, actor_id BIGINT NOT NULL REFERENCES actors(actor_id), movie_id BIGINT NOT NULL REFERENCES movies(movie_id), role VARCHAR);
CREATE TABLE collection (movie_id BIGINT PRIMARY KEY REFERENCES movies(movie_id), location VARCHAR NOT NULL, personal_rating DOUBLE CHECK(personal_rating BETWEEN 0 AND 10));
