/*
migrations are executed in order. the current version number is stored in the DB with PRAGMA user_version = <user_version>.
on application start, it will execute this from <user_version>..<latest index>.
 */
-- 2024.09.04 init --
create table jobs (
    id integer primary key autoincrement,
    status text not null default "",
    error text not null default "",
    operation text not null,
    time timestamp not null,
    use_mbid text not null default "",
    source_path text not null,
    dest_path text not null default "",
    search_result jsonb
);

create index idx_jobs_status on jobs (status);

create index idx_jobs_source_path on jobs (source_path);

-- 2024.12.23 add column research links --
alter table jobs
    add column research_links jsonb;

-- 2024.04.01 add confirm bool --
alter table jobs
    add column confirm boolean default false;

-- 2025.09.14 add updated_time timestamp --
alter table jobs
    add column updated_time timestamp;
