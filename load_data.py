"""Load the four supplied CSV files into a new DuckDB database."""
from pathlib import Path
import logging
import time
import tracemalloc
import duckdb
import polars as pl

ROOT = Path(__file__).resolve().parent
SCHEMAS = {
    'movies': {'movie_id': pl.Int64, 'name': pl.String, 'year': pl.Int32, 'rank': pl.Float64},
    'actors': {'actor_id': pl.Int64, 'first_name': pl.String, 'last_name': pl.String, 'gender': pl.String},
    'genres': {'movie_id': pl.Int64, 'genre': pl.String},
    'roles': {'actor_id': pl.Int64, 'movie_id': pl.Int64, 'role': pl.String},
}


def csv_path(table):
    """Return the expected location of one course-provided CSV file."""
    return ROOT / 'data' / f'aan520_w5a_{table}2000After.csv'


def load_database():
    # Check the CSV files before creating the database, so a missing file
    # does not leave behind an empty movies.duckdb that blocks the next run.
    missing = [csv_path(table).name for table in SCHEMAS if not csv_path(table).exists()]
    if missing:
        raise SystemExit('Missing data files in data/: ' + ', '.join(missing)
                         + '. Download them from the course assignment page (see README).')
    target = ROOT / 'movies.duckdb'
    if target.exists():
        raise FileExistsError('Database already exists. Back it up before rebuilding.')
    start = time.perf_counter()
    tracemalloc.start()
    logging.basicConfig(level=logging.INFO, format='%(levelname)s: %(message)s')
    with duckdb.connect(str(target)) as db:
        db.execute('BEGIN TRANSACTION')
        try:
            db.execute((ROOT / 'schema.sql').read_text())
            for table, schema in SCHEMAS.items():
                path = csv_path(table)
                frame = pl.read_csv(path, schema_overrides=schema).select(list(schema))
                db.register('source', frame.to_arrow())
                projection = 'row_number() OVER (), actor_id, movie_id, role' if table == 'roles' else ', '.join(schema)
                db.execute(f'INSERT INTO {table} SELECT {projection} FROM source')
                db.unregister('source')
                logging.info('%s: %s rows', table, frame.height)
            db.execute('COMMIT')
        except Exception:
            db.execute('ROLLBACK')
            raise
    _, peak = tracemalloc.get_traced_memory()
    tracemalloc.stop()
    logging.info('Elapsed: %.3f s; Python-tracked peak: %.2f MiB', time.perf_counter()-start, peak/1024**2)


if __name__ == '__main__':
    load_database()
