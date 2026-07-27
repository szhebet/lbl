--
-- PostgreSQL database dump
--

\restrict O3uU6yjLFzTx7YUO66ZUwYzYfeElcujj6zo4tr8Hdf1pviNlZsoI02alJjSq0S9

-- Dumped from database version 18.4 (Ubuntu 18.4-0ubuntu0.26.04.1)
-- Dumped by pg_dump version 18.4 (Ubuntu 18.4-0ubuntu0.26.04.1)

SET statement_timeout = 0;
SET lock_timeout = 0;
SET idle_in_transaction_session_timeout = 0;
SET transaction_timeout = 0;
SET client_encoding = 'UTF8';
SET standard_conforming_strings = on;
SELECT pg_catalog.set_config('search_path', '', false);
SET check_function_bodies = false;
SET xmloption = content;
SET client_min_messages = warning;
SET row_security = off;

--
-- Name: pg_trgm; Type: EXTENSION; Schema: -; Owner: -
--

CREATE EXTENSION IF NOT EXISTS pg_trgm WITH SCHEMA public;


--
-- Name: EXTENSION pg_trgm; Type: COMMENT; Schema: -; Owner: -
--

COMMENT ON EXTENSION pg_trgm IS 'text similarity measurement and index searching based on trigrams';


--
-- Name: uuid-ossp; Type: EXTENSION; Schema: -; Owner: -
--

CREATE EXTENSION IF NOT EXISTS "uuid-ossp" WITH SCHEMA public;


--
-- Name: EXTENSION "uuid-ossp"; Type: COMMENT; Schema: -; Owner: -
--

COMMENT ON EXTENSION "uuid-ossp" IS 'generate universally unique identifiers (UUIDs)';


--
-- Name: contributor_role; Type: TYPE; Schema: public; Owner: -
--

CREATE TYPE public.contributor_role AS ENUM (
    'author',
    'translator',
    'editor',
    'illustrator',
    'compiler',
    'preface_author',
    'commentary_author',
    'other'
);


--
-- Name: user_book_status; Type: TYPE; Schema: public; Owner: -
--

CREATE TYPE public.user_book_status AS ENUM (
    'Не заполнено',
    'Прочитано',
    'Читаю',
    'Отложил',
    'Бросил'
);


--
-- Name: normalize_search_field(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.normalize_search_field() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
    IF TG_OP = 'INSERT' OR TG_OP = 'UPDATE' THEN
        IF TG_TABLE_NAME = 'persons' THEN
            NEW.lower_fio := REPLACE(LOWER(COALESCE(NEW.last_name, '') || ' ' || COALESCE(NEW.first_name, '')), 'ё', 'е');
        ELSIF TG_TABLE_NAME = 'works' THEN
            NEW.lower_original_title := REPLACE(LOWER(NEW.original_title), 'ё', 'е');
        ELSIF TG_TABLE_NAME = 'editions' THEN
            NEW.lower_title := REPLACE(LOWER(NEW.title), 'ё', 'е');
        END IF;
    END IF;
    RETURN NEW;
END;
$$;


--
-- Name: sync_readlist_status_to_userbooks(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.sync_readlist_status_to_userbooks() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
    IF pg_trigger_depth() > 1 THEN
        RETURN NEW;
    END IF;
    IF NEW.book_id IS NOT NULL AND OLD.status IS DISTINCT FROM NEW.status THEN
        INSERT INTO user_books (user_id, edition_id, status)
        VALUES (NEW.user_id, NEW.book_id, NEW.status)
        ON CONFLICT (user_id, edition_id) DO UPDATE SET
            status = NEW.status,
            updated_at = CURRENT_TIMESTAMP;
    END IF;
    RETURN NEW;
END;
$$;


--
-- Name: sync_userbooks_status_to_readlist(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.sync_userbooks_status_to_readlist() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
    IF pg_trigger_depth() > 1 THEN
        RETURN NEW;
    END IF;
    IF OLD.status IS DISTINCT FROM NEW.status THEN
        UPDATE read_list
        SET status = NEW.status
        WHERE user_id = NEW.user_id AND book_id = NEW.edition_id;
    END IF;
    RETURN NEW;
END;
$$;


--
-- Name: update_timestamp(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.update_timestamp() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$;


SET default_tablespace = '';

SET default_table_access_method = heap;

--
-- Name: edition_files; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.edition_files (
    id integer NOT NULL,
    edition_id integer NOT NULL,
    format_id integer NOT NULL,
    file_path text NOT NULL,
    file_size bigint,
    file_hash character varying(64),
    page_count integer,
    word_count integer,
    has_ocr boolean DEFAULT false,
    has_bookmarks boolean DEFAULT false,
    has_images boolean DEFAULT true,
    is_drm boolean DEFAULT false,
    is_primary boolean DEFAULT false,
    source_file_id integer,
    converter text,
    created_at timestamp without time zone DEFAULT now(),
    updated_at timestamp without time zone DEFAULT now()
);


--
-- Name: editions; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.editions (
    id integer NOT NULL,
    work_id integer NOT NULL,
    isbn character varying(50),
    ean character varying(13),
    udc character varying(50),
    bbk character varying(50),
    title text NOT NULL,
    language character varying(3),
    publisher text,
    year integer,
    city text,
    pages integer,
    series text,
    series_number character varying(50),
    annotation text,
    source character varying(255),
    is_complete boolean DEFAULT true,
    quality character varying(20) DEFAULT 'good'::character varying,
    on_shelf boolean DEFAULT false,
    shelf_order integer DEFAULT 0,
    created_at timestamp without time zone DEFAULT now(),
    updated_at timestamp without time zone DEFAULT now(),
    upload_date timestamp without time zone DEFAULT now(),
    cover_path text,
    lower_title text,
    search_vector tsvector,
    uploaded_by integer
);


--
-- Name: formats; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.formats (
    id integer NOT NULL,
    name character varying(50) NOT NULL,
    extension character varying(10) NOT NULL,
    mime_type character varying(100),
    category character varying(50) NOT NULL,
    is_reflowable boolean DEFAULT true,
    is_editable boolean DEFAULT false
);


--
-- Name: genres; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.genres (
    id integer NOT NULL,
    name text NOT NULL,
    parent_id integer,
    description text
);


--
-- Name: persons; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.persons (
    id integer NOT NULL,
    first_name text,
    middle_name text,
    last_name text NOT NULL,
    pseudonym text,
    birth_date date,
    death_date date,
    biography text,
    photo_url text,
    created_at timestamp without time zone DEFAULT now(),
    lower_fio character varying(510)
);


--
-- Name: reading_progress; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.reading_progress (
    id integer NOT NULL,
    edition_id integer,
    current_position integer DEFAULT 0,
    total_positions integer,
    percentage real DEFAULT 0,
    device character varying(100),
    started_at timestamp without time zone,
    finished_at timestamp without time zone,
    rating integer,
    notes text,
    updated_at timestamp without time zone DEFAULT now(),
    CONSTRAINT reading_progress_rating_check CHECK (((rating >= 1) AND (rating <= 5)))
);


--
-- Name: users; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.users (
    id integer NOT NULL,
    username character varying(100) NOT NULL,
    password_hash character varying(255) NOT NULL,
    email character varying(255),
    role character varying(20) DEFAULT 'viewer'::character varying NOT NULL,
    created_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP,
    updated_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP
);


--
-- Name: work_contributors; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.work_contributors (
    work_id integer NOT NULL,
    person_id integer NOT NULL,
    role public.contributor_role NOT NULL
);


--
-- Name: work_genres; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.work_genres (
    work_id integer NOT NULL,
    genre_id integer NOT NULL
);


--
-- Name: works; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.works (
    id integer NOT NULL,
    original_title text NOT NULL,
    original_language character varying(3),
    first_published integer,
    work_type character varying(50) DEFAULT 'novel'::character varying,
    annotation text,
    word_count integer,
    created_at timestamp without time zone DEFAULT now(),
    updated_at timestamp without time zone DEFAULT now(),
    lower_original_title text,
    search_vector tsvector
);


--
-- Name: book_details; Type: VIEW; Schema: public; Owner: -
--

CREATE VIEW public.book_details AS
 SELECT w.id AS work_id,
    w.original_title,
    w.original_language,
    w.first_published,
    w.work_type,
    e.id AS edition_id,
    e.title AS edition_title,
    e.language AS edition_language,
    e.isbn,
    e.publisher,
    e.year,
    e.pages,
    e.series,
    e.series_number,
    e.quality,
    e.on_shelf,
    e.shelf_order,
    ( SELECT string_agg(((p.last_name || ' '::text) || p.first_name), ', '::text) AS string_agg
           FROM (public.work_contributors wc
             JOIN public.persons p ON ((p.id = wc.person_id)))
          WHERE ((wc.work_id = w.id) AND (wc.role = 'author'::public.contributor_role))) AS authors,
    ( SELECT string_agg(((p.last_name || ' '::text) || p.first_name), ', '::text) AS string_agg
           FROM (public.work_contributors wc
             JOIN public.persons p ON ((p.id = wc.person_id)))
          WHERE ((wc.work_id = w.id) AND (wc.role = 'translator'::public.contributor_role))) AS translators,
    ( SELECT string_agg(g.name, ', '::text) AS string_agg
           FROM (public.work_genres wg
             JOIN public.genres g ON ((g.id = wg.genre_id)))
          WHERE (wg.work_id = w.id)) AS genres,
    ( SELECT string_agg((f.name)::text, ', '::text ORDER BY (f.name)::text) AS string_agg
           FROM (public.edition_files ef
             JOIN public.formats f ON ((f.id = ef.format_id)))
          WHERE (ef.edition_id = e.id)) AS available_formats,
    ( SELECT count(*) AS count
           FROM public.edition_files ef
          WHERE (ef.edition_id = e.id)) AS format_count,
    ( SELECT ef.file_path
           FROM public.edition_files ef
          WHERE ((ef.edition_id = e.id) AND (ef.is_primary = true))
         LIMIT 1) AS primary_file_path,
    rp.percentage AS reading_progress,
    rp.rating,
    rp.finished_at,
    e.upload_date,
    e.created_at,
    e.updated_at,
    e.uploaded_by,
    u.username AS uploaded_by_username
   FROM (((public.works w
     JOIN public.editions e ON ((e.work_id = w.id)))
     LEFT JOIN public.reading_progress rp ON ((rp.edition_id = e.id)))
     LEFT JOIN public.users u ON ((u.id = e.uploaded_by)));


--
-- Name: collection_items; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.collection_items (
    id integer NOT NULL,
    collection_id integer,
    edition_id integer,
    sort_order integer DEFAULT 0,
    added_at timestamp without time zone DEFAULT now()
);


--
-- Name: collection_items_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.collection_items_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: collection_items_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.collection_items_id_seq OWNED BY public.collection_items.id;


--
-- Name: collections; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.collections (
    id integer NOT NULL,
    name text NOT NULL,
    description text,
    is_public boolean DEFAULT false,
    created_at timestamp without time zone DEFAULT now(),
    updated_at timestamp without time zone DEFAULT now()
);


--
-- Name: collections_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.collections_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: collections_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.collections_id_seq OWNED BY public.collections.id;


--
-- Name: conversion_log; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.conversion_log (
    id integer NOT NULL,
    source_file_id integer,
    target_file_id integer,
    converter character varying(50) NOT NULL,
    options jsonb,
    started_at timestamp without time zone DEFAULT now(),
    finished_at timestamp without time zone,
    status character varying(20) DEFAULT 'running'::character varying,
    error_message text,
    original_size bigint,
    result_size bigint
);


--
-- Name: conversion_log_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.conversion_log_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: conversion_log_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.conversion_log_id_seq OWNED BY public.conversion_log.id;


--
-- Name: db_version; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.db_version (
    version character varying(20) NOT NULL,
    updated_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP
);


--
-- Name: duplicate_candidates; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.duplicate_candidates (
    id integer NOT NULL,
    edition_id_1 integer NOT NULL,
    edition_id_2 integer NOT NULL,
    match_type character varying(50) NOT NULL,
    confidence real DEFAULT 0.0,
    details jsonb,
    is_confirmed boolean DEFAULT false,
    is_merged boolean DEFAULT false,
    created_at timestamp without time zone DEFAULT now(),
    resolved_at timestamp without time zone
);


--
-- Name: duplicate_candidates_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.duplicate_candidates_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: duplicate_candidates_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.duplicate_candidates_id_seq OWNED BY public.duplicate_candidates.id;


--
-- Name: edition_files_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.edition_files_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: edition_files_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.edition_files_id_seq OWNED BY public.edition_files.id;


--
-- Name: edition_tags; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.edition_tags (
    edition_id integer NOT NULL,
    tag_id integer NOT NULL
);


--
-- Name: editions_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.editions_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: editions_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.editions_id_seq OWNED BY public.editions.id;


--
-- Name: format_summary; Type: VIEW; Schema: public; Owner: -
--

CREATE VIEW public.format_summary AS
 SELECT f.name,
    f.extension,
    f.category,
    count(ef.id) AS file_count,
    count(DISTINCT ef.edition_id) AS unique_editions,
    sum(ef.file_size) AS total_size_bytes,
    round(avg(ef.file_size), 0) AS avg_size_bytes
   FROM (public.formats f
     LEFT JOIN public.edition_files ef ON ((ef.format_id = f.id)))
  GROUP BY f.id, f.name, f.extension, f.category
  ORDER BY (count(ef.id)) DESC;


--
-- Name: formats_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.formats_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: formats_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.formats_id_seq OWNED BY public.formats.id;


--
-- Name: genres_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.genres_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: genres_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.genres_id_seq OWNED BY public.genres.id;


--
-- Name: import_sessions; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.import_sessions (
    id integer NOT NULL,
    source_type character varying(50) NOT NULL,
    source_path text,
    started_at timestamp without time zone DEFAULT now(),
    finished_at timestamp without time zone,
    total_processed integer DEFAULT 0,
    new_works integer DEFAULT 0,
    new_editions integer DEFAULT 0,
    new_files integer DEFAULT 0,
    duplicates_found integer DEFAULT 0,
    errors text[],
    status character varying(20) DEFAULT 'running'::character varying
);


--
-- Name: import_sessions_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.import_sessions_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: import_sessions_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.import_sessions_id_seq OWNED BY public.import_sessions.id;


--
-- Name: languages; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.languages (
    code character varying(3) NOT NULL,
    name character varying(100) NOT NULL,
    native_name character varying(100)
);


--
-- Name: persons_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.persons_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: persons_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.persons_id_seq OWNED BY public.persons.id;


--
-- Name: read_list; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.read_list (
    listname text DEFAULT 'default'::text NOT NULL,
    bookname text DEFAULT ''::text NOT NULL,
    author text DEFAULT ''::text NOT NULL,
    priority integer DEFAULT 0 NOT NULL,
    author_id integer,
    book_id integer,
    user_id integer NOT NULL,
    comment text DEFAULT ''::text NOT NULL,
    status public.user_book_status DEFAULT 'Не заполнено'::public.user_book_status NOT NULL,
    created_at timestamp without time zone DEFAULT now(),
    id uuid CONSTRAINT read_list_new_id_not_null NOT NULL,
    updated_at timestamp without time zone DEFAULT now(),
    synced_at timestamp without time zone,
    deleted boolean DEFAULT false NOT NULL,
    looking_for text DEFAULT 'Нет'::text NOT NULL
);


--
-- Name: reading_progress_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.reading_progress_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: reading_progress_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.reading_progress_id_seq OWNED BY public.reading_progress.id;


--
-- Name: refresh_tokens; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.refresh_tokens (
    id integer NOT NULL,
    user_id integer NOT NULL,
    token_hash character varying(64) NOT NULL,
    device_name character varying(255) DEFAULT ''::character varying,
    device_fingerprint character varying(255) DEFAULT ''::character varying,
    created_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP
);


--
-- Name: refresh_tokens_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.refresh_tokens_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: refresh_tokens_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.refresh_tokens_id_seq OWNED BY public.refresh_tokens.id;


--
-- Name: settings; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.settings (
    key character varying(100) NOT NULL,
    value text DEFAULT ''::text NOT NULL,
    updated_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP
);


--
-- Name: shelf_tokens; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.shelf_tokens (
    id integer NOT NULL,
    token character varying(64) NOT NULL,
    edition_id integer NOT NULL,
    created_at timestamp without time zone DEFAULT now()
);


--
-- Name: shelf_tokens_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.shelf_tokens_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: shelf_tokens_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.shelf_tokens_id_seq OWNED BY public.shelf_tokens.id;


--
-- Name: tags; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.tags (
    id integer NOT NULL,
    name text NOT NULL,
    color character varying(7),
    description text
);


--
-- Name: tags_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.tags_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: tags_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.tags_id_seq OWNED BY public.tags.id;


--
-- Name: toc_entries; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.toc_entries (
    id integer NOT NULL,
    edition_id integer NOT NULL,
    parent_id integer,
    level integer DEFAULT 1 NOT NULL,
    title text NOT NULL,
    "position" integer,
    sort_order integer DEFAULT 0 NOT NULL
);


--
-- Name: toc_entries_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.toc_entries_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: toc_entries_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.toc_entries_id_seq OWNED BY public.toc_entries.id;


--
-- Name: user_books; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.user_books (
    id integer NOT NULL,
    user_id integer NOT NULL,
    edition_id integer NOT NULL,
    status public.user_book_status DEFAULT 'Не заполнено'::public.user_book_status NOT NULL,
    review text DEFAULT ''::text NOT NULL,
    rating integer,
    date_started date,
    date_read date,
    created_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP,
    updated_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT user_books_rating_check CHECK (((rating >= 1) AND (rating <= 10)))
);


--
-- Name: user_books_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.user_books_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: user_books_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.user_books_id_seq OWNED BY public.user_books.id;


--
-- Name: user_devices; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.user_devices (
    id integer NOT NULL,
    user_id integer NOT NULL,
    device_name character varying(255) NOT NULL,
    device_fingerprint character varying(255) NOT NULL,
    created_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP
);


--
-- Name: user_devices_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.user_devices_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: user_devices_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.user_devices_id_seq OWNED BY public.user_devices.id;


--
-- Name: users_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.users_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: users_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.users_id_seq OWNED BY public.users.id;


--
-- Name: works_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

CREATE SEQUENCE public.works_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: works_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: -
--

ALTER SEQUENCE public.works_id_seq OWNED BY public.works.id;


--
-- Name: collection_items id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.collection_items ALTER COLUMN id SET DEFAULT nextval('public.collection_items_id_seq'::regclass);


--
-- Name: collections id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.collections ALTER COLUMN id SET DEFAULT nextval('public.collections_id_seq'::regclass);


--
-- Name: conversion_log id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.conversion_log ALTER COLUMN id SET DEFAULT nextval('public.conversion_log_id_seq'::regclass);


--
-- Name: duplicate_candidates id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.duplicate_candidates ALTER COLUMN id SET DEFAULT nextval('public.duplicate_candidates_id_seq'::regclass);


--
-- Name: edition_files id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.edition_files ALTER COLUMN id SET DEFAULT nextval('public.edition_files_id_seq'::regclass);


--
-- Name: editions id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.editions ALTER COLUMN id SET DEFAULT nextval('public.editions_id_seq'::regclass);


--
-- Name: formats id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.formats ALTER COLUMN id SET DEFAULT nextval('public.formats_id_seq'::regclass);


--
-- Name: genres id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.genres ALTER COLUMN id SET DEFAULT nextval('public.genres_id_seq'::regclass);


--
-- Name: import_sessions id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.import_sessions ALTER COLUMN id SET DEFAULT nextval('public.import_sessions_id_seq'::regclass);


--
-- Name: persons id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.persons ALTER COLUMN id SET DEFAULT nextval('public.persons_id_seq'::regclass);


--
-- Name: reading_progress id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.reading_progress ALTER COLUMN id SET DEFAULT nextval('public.reading_progress_id_seq'::regclass);


--
-- Name: refresh_tokens id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.refresh_tokens ALTER COLUMN id SET DEFAULT nextval('public.refresh_tokens_id_seq'::regclass);


--
-- Name: shelf_tokens id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.shelf_tokens ALTER COLUMN id SET DEFAULT nextval('public.shelf_tokens_id_seq'::regclass);


--
-- Name: tags id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.tags ALTER COLUMN id SET DEFAULT nextval('public.tags_id_seq'::regclass);


--
-- Name: toc_entries id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.toc_entries ALTER COLUMN id SET DEFAULT nextval('public.toc_entries_id_seq'::regclass);


--
-- Name: user_books id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_books ALTER COLUMN id SET DEFAULT nextval('public.user_books_id_seq'::regclass);


--
-- Name: user_devices id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_devices ALTER COLUMN id SET DEFAULT nextval('public.user_devices_id_seq'::regclass);


--
-- Name: users id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.users ALTER COLUMN id SET DEFAULT nextval('public.users_id_seq'::regclass);


--
-- Name: works id; Type: DEFAULT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.works ALTER COLUMN id SET DEFAULT nextval('public.works_id_seq'::regclass);


--
-- Data for Name: collection_items; Type: TABLE DATA; Schema: public; Owner: -
--

COPY public.collection_items (id, collection_id, edition_id, sort_order, added_at) FROM stdin;
\.


--
-- Data for Name: collections; Type: TABLE DATA; Schema: public; Owner: -
--

COPY public.collections (id, name, description, is_public, created_at, updated_at) FROM stdin;
\.


--
-- Data for Name: conversion_log; Type: TABLE DATA; Schema: public; Owner: -
--

COPY public.conversion_log (id, source_file_id, target_file_id, converter, options, started_at, finished_at, status, error_message, original_size, result_size) FROM stdin;
\.


--
-- Data for Name: db_version; Type: TABLE DATA; Schema: public; Owner: -
--

COPY public.db_version (version, updated_at) FROM stdin;
4.3	2026-07-27 10:12:33.220758
4.3	2026-07-27 10:12:33.220758
\.


--
-- Data for Name: duplicate_candidates; Type: TABLE DATA; Schema: public; Owner: -
--

COPY public.duplicate_candidates (id, edition_id_1, edition_id_2, match_type, confidence, details, is_confirmed, is_merged, created_at, resolved_at) FROM stdin;
\.


--
-- Data for Name: edition_files; Type: TABLE DATA; Schema: public; Owner: -
--

COPY public.edition_files (id, edition_id, format_id, file_path, file_size, file_hash, page_count, word_count, has_ocr, has_bookmarks, has_images, is_drm, is_primary, source_file_id, converter, created_at, updated_at) FROM stdin;
101	101	1	bookarch/00002/Ultima_Tuleev__ili_Dao_vyborov.zip	179139	8ead47eac2f6436c9cfc6dad7cf56879ff3320a58bd476307b79485afc1147a4	\N	\N	f	f	t	f	t	\N	\N	2026-07-10 17:16:16.314597	2026-07-10 17:16:16.314597
102	102	1	bookarch/00002/Who_by_fire.zip	46095	d2f989323f90c030a9ce001f1248aa1d0b6c25e092ff43521f63cf7beec50086	\N	\N	f	f	t	f	t	\N	\N	2026-07-10 17:16:16.320947	2026-07-10 17:16:16.320947
103	103	1	bookarch/00002/t.zip	341194	3d6ec2327c2ab6b7bdd3572956ddc2fd156aff621bb2e617f9ddc83a2e5f4045	\N	\N	f	f	t	f	t	\N	\N	2026-07-10 17:16:16.368714	2026-07-10 17:16:16.368714
104	104	1	bookarch/00002/Akiko.zip	13243	fdd0b7f4db92953aee0b9af62ba33f0280bc4fcfa15c7672a1615a91afe8ccf3	\N	\N	f	f	t	f	t	\N	\N	2026-07-10 17:16:16.376014	2026-07-10 17:16:16.376014
105	105	1	bookarch/00002/Ampir__V.zip	407208	c7ba6d6aeb714ec6fb2dba2f73929a1a2b8ddb4bf50fec9d8573781b585e02a3	\N	\N	f	f	t	f	t	\N	\N	2026-07-10 17:16:16.419499	2026-07-10 17:16:16.419499
106	106	1	bookarch/00002/Ananasnaya_voda_dlya_prekrasnoy_damy.zip	294728	2861f5fc8de75b1638972c9dd5d807a0895114e3abd7773c4e06fd6abf5e6d2b	\N	\N	f	f	t	f	t	\N	\N	2026-07-10 17:16:16.456042	2026-07-10 17:16:16.456042
107	107	1	bookarch/00002/Buben_verkhnego_mira.zip	15232	002597821084035c25001a198deb6b7f5a3e878e03fde33639400b1170f1b1a2	\N	\N	f	f	t	f	t	\N	\N	2026-07-10 17:16:16.46316	2026-07-10 17:16:16.46316
108	108	1	bookarch/00002/Buben_nizhnego_mira.zip	7056	b829a0bb66f4e218473ec2824446826ec6c78fdc87fd0965da83a8cb192a7c78	\N	\N	f	f	t	f	t	\N	\N	2026-07-10 17:16:16.469526	2026-07-10 17:16:16.469526
109	109	1	bookarch/00002/Betman_Apollo.zip	581692	7a3a5e2b3dc4f36c44785308ac37366b2c36b7647f2c538a8f5e6e57fcfdba2c	\N	\N	f	f	t	f	t	\N	\N	2026-07-10 17:16:16.528354	2026-07-10 17:16:16.528354
110	110	1	bookarch/00002/Vesti_iz_Nepala.zip	48666	856eaee9359dfadacdf8ee75b7b6b57027b152c83599a5f197157066ffb0df2c	\N	\N	f	f	t	f	t	\N	\N	2026-07-10 17:16:16.537958	2026-07-10 17:16:16.537958
111	111	1	bookarch/00002/Viktor_Pelevin_sprashivaet_PRov.zip	6929	6e502735ed26b49a8be703b1ca1c29596f3dc6453a6adeb52ae76ba1409753cf	\N	\N	f	f	t	f	t	\N	\N	2026-07-10 17:16:16.543519	2026-07-10 17:16:16.543519
112	112	1	bookarch/00002/Vodonapornaya_bashnya.zip	11406	eebf4e357780f8e7f1dda44d454b02561c28c0daa316970e53bf5747cdd87172	\N	\N	f	f	t	f	t	\N	\N	2026-07-10 17:16:16.549714	2026-07-10 17:16:16.549714
113	113	1	bookarch/00002/Vse_rasskazy__Sbornik.zip	612740	1e064e26e518e5474f8ab776497768446e63d14478e367a2c75112043bbe7f44	\N	\N	f	f	t	f	t	\N	\N	2026-07-10 17:16:16.616765	2026-07-10 17:16:16.616765
114	114	1	bookarch/00002/Vstroennyy_napominatel.zip	5137	6fafae0ad14009a9abea6d4db4a6fe8ba07fa4a6aebd63606d142d47332f18da	\N	\N	f	f	t	f	t	\N	\N	2026-07-10 17:16:16.623366	2026-07-10 17:16:16.623366
115	115	1	bookarch/00002/GKChP_kak_tetragrammaton.zip	5221	d60df98b36233fe08bf75b28dbee0676788b5fc87370a6197dab4d581290e49d	\N	\N	f	f	t	f	t	\N	\N	2026-07-10 17:16:16.62855	2026-07-10 17:16:16.62855
116	116	1	bookarch/00002/Gadanie_na_runakh_ili_runicheskiy_orakul_Ralfa_Bluma.zip	44854	bfc78740dd753b08ca8b562d7e6968be2d1845c62ebc580fd14bcc06159d58c3	\N	\N	f	f	t	f	t	\N	\N	2026-07-10 17:16:16.636659	2026-07-10 17:16:16.636659
117	117	1	bookarch/00002/Grecheskiy_variant.zip	13089	4d538e76cab6a28cf190d4672b86213506884640aa6458deb127d0f1721ed1bf	\N	\N	f	f	t	f	t	\N	\N	2026-07-10 17:16:16.642782	2026-07-10 17:16:16.642782
118	118	1	bookarch/00002/DPP__NN___sbornik.zip	1855294	635e2320b4d38cfee87f52fcd9fd084810289efea40a939cfc5b4c7be9ccf323	\N	\N	f	f	t	f	t	\N	\N	2026-07-10 17:16:16.722467	2026-07-10 17:16:16.722467
119	119	1	bookarch/00002/Devyatyy_son_Very_Pavlovny.zip	24436	f42e3b96fdcdbc203186b52dd9a8fb200ff5764df9bc51f6b6accd8290c733ee	\N	\N	f	f	t	f	t	\N	\N	2026-07-10 17:16:16.731626	2026-07-10 17:16:16.731626
120	120	1	bookarch/00002/Den_buldozerista.zip	30384	8e5baaf51bbd3a5cdda6c7b914308257126d84caa3b2e7a721956d8a5d3a4a70	\N	\N	f	f	t	f	t	\N	\N	2026-07-10 17:16:16.740215	2026-07-10 17:16:16.740215
214	214	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-12 12:18:03.031074	2026-07-12 12:18:03.031074
121	121	1	bookarch/00002/Dzhon_Faulz_i_tragediya_russkogo_liberalizma.zip	6478	f08e3ac488ef5f0e6d9a0c2f4f1dcc47509b06b6bff7e44104e184b239afa6d0	\N	\N	f	f	t	f	t	\N	\N	2026-07-10 17:16:16.746196	2026-07-10 17:16:16.746196
122	122	1	bookarch/00002/Dialektika_Perekhodnogo_Perioda_Iz_Niotkuda_V_Nikuda.zip	317666	481bdf515198d7c589f0ed061c53568afe39712f3c26fec718e2d38f03e9e3ba	\N	\N	f	f	t	f	t	\N	\N	2026-07-10 17:16:16.788	2026-07-10 17:16:16.788
123	123	1	bookarch/00002/Zheltaya_strela.zip	80640	8f0acc66da52bd1cfc453b29f6533eac79cfebf6d29a2663926644690bc3217f	\N	\N	f	f	t	f	t	\N	\N	2026-07-10 17:16:16.799785	2026-07-10 17:16:16.799785
124	124	1	bookarch/00002/Zhizn_i_priklyucheniya_saraya_nomer_XII.zip	12644	c24b96ca50764e86e4d7a24d7f4b2d580bb61e9032ebfc96e572f37c4612030d	\N	\N	f	f	t	f	t	\N	\N	2026-07-10 17:16:16.805983	2026-07-10 17:16:16.805983
125	125	1	bookarch/00002/Zhizn_nasekomykh.zip	189673	88649798f243da48c58a28b91938b48a5ba6de8d39bca5db79f3510e162a9eac	\N	\N	f	f	t	f	t	\N	\N	2026-07-10 17:16:16.83182	2026-07-10 17:16:16.83182
126	126	1	bookarch/00002/Zal_poyushchikh_kariatid.zip	161418	3335e1bb4199fd4770cfb73d6ab26c1930f8059cb289f7fd891f3adb2ea1e8fc	\N	\N	f	f	t	f	t	\N	\N	2026-07-10 17:16:16.84748	2026-07-10 17:16:16.84748
127	127	1	bookarch/00002/Zapis_o_poiske_vetra.zip	176197	94f4a6da351bea5a49f1e2c62b96ceaa3b150e3448f41e7ba677a0e0cfd188c1	\N	\N	f	f	t	f	t	\N	\N	2026-07-10 17:16:16.858031	2026-07-10 17:16:16.858031
128	128	1	bookarch/00002/Zatvornik_i_Shestipalyy.zip	68235	1c231c598103c200ea867ac83767c5536f9435a055888868d3322d3685436d95	\N	\N	f	f	t	f	t	\N	\N	2026-07-10 17:16:16.868426	2026-07-10 17:16:16.868426
129	129	1	bookarch/00002/Zigmund_v_kafe.zip	8176	519c45c8fc7ca5b8c28e58f1c87a5b39a3f49ea564fe98db87efad07d4b1624e	\N	\N	f	f	t	f	t	\N	\N	2026-07-10 17:16:16.874251	2026-07-10 17:16:16.874251
130	130	1	bookarch/00002/Zombifikatsiya__Opyt_sravnitelnoy_antropologii.zip	29566	1cea6c9b5646bca23075f1658a6da29f879448b71c9077480266f65d68e8acc9	\N	\N	f	f	t	f	t	\N	\N	2026-07-10 17:16:16.883165	2026-07-10 17:16:16.883165
131	131	1	bookarch/00002/Ivan_Kublakhanov.zip	11006	4400a824bbb7dce6cccb4470766fc2d4cdca2c7e9277dc53efda57532f2b447a	\N	\N	f	f	t	f	t	\N	\N	2026-07-10 17:16:16.890637	2026-07-10 17:16:16.890637
132	132	1	bookarch/00003/Ikstlan___Petushki.zip	6027	0fef59f313966b3b9cd6a471419e891560e5ca109f0faf39ecc8f26f962a4d86	\N	\N	f	f	t	f	t	\N	\N	2026-07-10 17:16:16.896876	2026-07-10 17:16:16.896876
133	133	1	bookarch/00003/Imena_oligarkhov_na_karte_Rodiny.zip	57550	5067d64d7a9630da94f9d43d9394c587c041536cfc3122c24575caf7c420b648	\N	\N	f	f	t	f	t	\N	\N	2026-07-10 17:16:16.903472	2026-07-10 17:16:16.903472
134	134	1	bookarch/00003/Intervyu_s_Viktorom_Pelevinym__2.zip	5740	0fe0b7c94402a9b567da2ea64f2369305237edb5b00a31065beaea309e7e30a5	\N	\N	f	f	t	f	t	\N	\N	2026-07-10 17:16:16.907737	2026-07-10 17:16:16.907737
135	135	1	bookarch/00003/Intervyu_s_Viktorom_Pelevinym.zip	4636	d1df2ecd37a886bc45621da51b3bdee2084ad9a969d263b99e3faaa2a9498218	\N	\N	f	f	t	f	t	\N	\N	2026-07-10 17:16:16.911941	2026-07-10 17:16:16.911941
136	136	1	bookarch/00003/Kod_Mira.zip	5032	4fc7e9a5c411aa34ec04f29aa3714b7db08c24ad4084afd11d8b6e9b9809aebc	\N	\N	f	f	t	f	t	\N	\N	2026-07-10 17:16:16.916201	2026-07-10 17:16:16.916201
137	137	1	bookarch/00003/Koldun_Ignat_i_lyudi__sbornik.zip	181382	58f7b3df5683154f99b69f5446505f0f9afa0ca8fad5b1b031c8e1524a280a61	\N	\N	f	f	t	f	t	\N	\N	2026-07-10 17:16:16.932095	2026-07-10 17:16:16.932095
138	138	1	bookarch/00003/Koldun_Ignat_i_lyudi.zip	2919	da77d407feab5f62f86ecee6801699e52a416a0a9e3ae6e6f4f41b049b8eb9e3	\N	\N	f	f	t	f	t	\N	\N	2026-07-10 17:16:16.936702	2026-07-10 17:16:16.936702
139	139	1	bookarch/00003/Kormlenie_krokodila_Khufu.zip	21203	f5a098db80d9ae460af6045cdff49a13b3bf3b5c0829696bc34dea3f3d2e65a3	\N	\N	f	f	t	f	t	\N	\N	2026-07-10 17:16:16.942997	2026-07-10 17:16:16.942997
140	140	1	bookarch/00003/Kratkaya_istoriya_peyntbola_v_Moskve.zip	19922	03494ff7c15665204c22c652526d4869e4ba1f78f5c84c97d054bbe2292eb52d	\N	\N	f	f	t	f	t	\N	\N	2026-07-10 17:16:16.950425	2026-07-10 17:16:16.950425
141	141	1	bookarch/00003/Lunokhod.zip	11963	181d63a9558f106e43a5c1592504444631544b26344d5a69d9f8b41d31a66e99	\N	\N	f	f	t	f	t	\N	\N	2026-07-10 17:16:16.9567	2026-07-10 17:16:16.9567
142	142	1	bookarch/00003/Lyubov_k_trem_tsukerbrinam.zip	472565	93afdb9fb96cf82064cfa745e68fe0bef42bfbb0e204e7d05e4000dc5e1f1a37	\N	\N	f	f	t	f	t	\N	\N	2026-07-10 17:16:16.984013	2026-07-10 17:16:16.984013
143	143	1	bookarch/00003/Makedonskaya_kritika_frantsuzskoy_mysli__Sbornik.zip	179795	77c185c9bfde9b0fc165721ec124afe193a3332563fcaf6a6e400267cce467dd	\N	\N	f	f	t	f	t	\N	\N	2026-07-10 17:16:17.000042	2026-07-10 17:16:17.000042
144	144	1	bookarch/00003/Mardongi.zip	6855	b74c9caf586dcd92ff0c32b18fca35471614657c3a158f8bbbfc10917f87d0cf	\N	\N	f	f	t	f	t	\N	\N	2026-07-10 17:16:17.005258	2026-07-10 17:16:17.005258
145	145	1	bookarch/00003/Mittelshpil.zip	30006	3e5e994486f072326f8e3d17cf2b3cd1da2cf0a7e536743a2d9da41d16dc2843	\N	\N	f	f	t	f	t	\N	\N	2026-07-10 17:16:17.013182	2026-07-10 17:16:17.013182
146	146	1	bookarch/00003/Moy_meskalitovyy_trip.zip	4976	698bc1f2351828d232f4cf1294d33631267b38255c6b0d0f671b73f1550356dc	\N	\N	f	f	t	f	t	\N	\N	2026-07-10 17:16:17.017912	2026-07-10 17:16:17.017912
147	147	1	bookarch/00003/Most__kotoryy_ya_khotel_pereyti.zip	2630	82da4b2ada9cfc803cbfbd768cd01dc6c06d098baeefa8c70ca5f678b221e29a	\N	\N	f	f	t	f	t	\N	\N	2026-07-10 17:16:17.02192	2026-07-10 17:16:17.02192
148	148	1	bookarch/00003/Muzyka_so_stolba.zip	16680	59a346a4cd7cc185b2a2185c6643c47814a86479806388702e9c63f286fe2cd5	\N	\N	f	f	t	f	t	\N	\N	2026-07-10 17:16:17.027413	2026-07-10 17:16:17.027413
149	149	1	bookarch/00003/Nizhnyaya_tundra.zip	20781	01ecb9116a2bd7d171115f91f8d0ca76800d3d2fbf6c2b6be76b53da80796f7d	\N	\N	f	f	t	f	t	\N	\N	2026-07-10 17:16:17.033701	2026-07-10 17:16:17.033701
150	150	1	bookarch/00003/Nika.zip	16952	1f1773696c897fe19d6552000b887a17fc785934b11ee9031edf076b03691023	\N	\N	f	f	t	f	t	\N	\N	2026-07-10 17:16:17.040046	2026-07-10 17:16:17.040046
151	151	1	bookarch/00003/Omon_Ra.zip	125720	97fc625ab0a9bc5384113155c6b5e30ef57432a13eb411074970df14b65aa532	\N	\N	f	f	t	f	t	\N	\N	2026-07-10 17:16:17.058808	2026-07-10 17:16:17.058808
152	152	1	bookarch/00003/Ontologiya_detstva.zip	12846	330ca4dd10e21462fbe0624c1176b8b5ad6c0a00e7f2e58a09e78301668fc798	\N	\N	f	f	t	f	t	\N	\N	2026-07-10 17:16:17.064766	2026-07-10 17:16:17.064766
153	153	1	bookarch/00003/Oruzhie_vozmezdiya.zip	13841	2d61cd7cd6f087d00ab82161f9ff42f180773b5532748d655f38209fae911ab2	\N	\N	f	f	t	f	t	\N	\N	2026-07-10 17:16:17.071346	2026-07-10 17:16:17.071346
154	154	1	bookarch/00003/Otkrovenie_Kregera.zip	7391	f559203a54b29892455ec99a64130addbcb92934fe5d28243c3d324022025304	\N	\N	f	f	t	f	t	\N	\N	2026-07-10 17:16:17.076533	2026-07-10 17:16:17.076533
155	155	1	bookarch/00003/Papakhi_na_bashnyakh.zip	17023	931a1449396561a7299208766beddf145462a15794af6f82baad449ea4393a27	\N	\N	f	f	t	f	t	\N	\N	2026-07-10 17:16:17.082963	2026-07-10 17:16:17.082963
156	156	1	bookarch/00003/Podzemnoe_nebo.zip	5363	a03fcd968cd8f0684bbaf3947be7a0e6548d87d23ac09544a0b6edca14fceb91	\N	\N	f	f	t	f	t	\N	\N	2026-07-10 17:16:17.087774	2026-07-10 17:16:17.087774
157	157	1	bookarch/00003/Poslednyaya_shutka_voina.zip	3863	6d4822ec4530a93793163a2cec405ffe3a343567b1ac039bdc2f305785fd415b	\N	\N	f	f	t	f	t	\N	\N	2026-07-10 17:16:17.092582	2026-07-10 17:16:17.092582
158	158	1	bookarch/00003/Prints_Gosplana.zip	48078	51ac918788bada72c39b43c443dce501eb2e9f3954827b40ea4a2a46c52d35de	\N	\N	f	f	t	f	t	\N	\N	2026-07-10 17:16:17.104327	2026-07-10 17:16:17.104327
159	159	1	bookarch/00003/Problema_vervolka_v_sredney_polose.zip	37792	5e7664bab63dae58a8416c325fe466d603d6606f5b15e4a86e4f05c6374dbfe3	\N	\N	f	f	t	f	t	\N	\N	2026-07-10 17:16:17.114427	2026-07-10 17:16:17.114427
160	160	1	bookarch/00003/Proiskhozhdenie_vidov.zip	12427	61840ed4ae66e2117b39f88fc061fa1126aab52128357177f406397aabc37c48	\N	\N	f	f	t	f	t	\N	\N	2026-07-10 17:16:17.119512	2026-07-10 17:16:17.119512
161	161	1	bookarch/00003/Prostranstvo_Fridmana.zip	104274	523d2b9eec05eac11254030d9225da2ede993a012f23ba7ce42760314f9a9aac	\N	\N	f	f	t	f	t	\N	\N	2026-07-10 17:16:17.126524	2026-07-10 17:16:17.126524
162	162	1	bookarch/00003/P5__Proshchalnye_pesni_politicheskikh_pigmeev_Pindostana.zip	215157	5941f499cebfbd62ece606ae6ba04cd03c84062c727a685e1a8a25d90ccfa418	\N	\N	f	f	t	f	t	\N	\N	2026-07-10 17:16:17.152326	2026-07-10 17:16:17.152326
163	163	1	bookarch/00003/Rekonstruktor.zip	8900	5601c628b4a4fa0b5382e0bb084afff61f83499994a4da4631037eefcc81b8d7	\N	\N	f	f	t	f	t	\N	\N	2026-07-10 17:16:17.157249	2026-07-10 17:16:17.157249
164	164	1	bookarch/00003/SSSR_Tayshou_Chzhuan.zip	15617	8e09cebd111a1a42dbb36d59fd968bf6fcd91fa069d1d48982c7366cfb9f45a0	\N	\N	f	f	t	f	t	\N	\N	2026-07-10 17:16:17.162833	2026-07-10 17:16:17.162833
165	165	1	bookarch/00003/Svet_gorizonta.zip	21903	6d9d93f02bbfe7d23f8ab448eb3d60a9404b1069fdaf3dcdabb0c3b9b1e70150	\N	\N	f	f	t	f	t	\N	\N	2026-07-10 17:16:17.169729	2026-07-10 17:16:17.169729
166	166	1	bookarch/00003/Svyatochnyy_kiberpank_117_dir.zip	15823	1c0120c9030360f496d2ef9b1c9be4b4f424f8624c2b5184bfee42fbb19273f4	\N	\N	f	f	t	f	t	\N	\N	2026-07-10 17:16:17.175617	2026-07-10 17:16:17.175617
167	167	1	bookarch/00003/Svyashchennaya_kniga_oborotnya.zip	306551	4419b708741ab65c761867ef589c0ebe4c700bfc2975054ff7f24df6fde84b3a	\N	\N	f	f	t	f	t	\N	\N	2026-07-10 17:16:17.217133	2026-07-10 17:16:17.217133
168	168	1	bookarch/00003/Siniy_fonar.zip	10505	17a59ee987695d90d5d846a768678d9a6b4b1f2899e1c9987126488dbaad1cbc	\N	\N	f	f	t	f	t	\N	\N	2026-07-10 17:16:17.223892	2026-07-10 17:16:17.223892
169	169	1	bookarch/00003/Siniy_fonar_1.zip	1293189	00316fa7245e29f1489ede0981807545aa9d44d952bc510fdb76e638b92a898c	\N	\N	f	f	t	f	t	\N	\N	2026-07-10 17:16:17.274981	2026-07-10 17:16:17.274981
170	170	1	bookarch/00003/Spi.zip	20155	73860dd08b5f86f6f85f5519338c80835e15a4fe13ba9b1809550d56339896c2	\N	\N	f	f	t	f	t	\N	\N	2026-07-10 17:16:17.282202	2026-07-10 17:16:17.282202
171	171	1	bookarch/00003/Taym-aut__ili_Vechernyaya_Moskva.zip	6469	1e34c7c31e86f318698f7317899aed3c25b49c0f19eecb7170ecd082be6a5502	\N	\N	f	f	t	f	t	\N	\N	2026-07-10 17:16:17.287425	2026-07-10 17:16:17.287425
172	172	1	bookarch/00003/Tarzanka.zip	17909	e9f52f84879904575232187cc491b882869bd869af1c1af40d932a8b695bf71f	\N	\N	f	f	t	f	t	\N	\N	2026-07-10 17:16:17.292974	2026-07-10 17:16:17.292974
173	173	1	bookarch/00003/Tkhagi.zip	80905	25c8414134f82769358fa08f40181c764cfc4097218fe49f94b8735a753d92c8	\N	\N	f	f	t	f	t	\N	\N	2026-07-10 17:16:17.301536	2026-07-10 17:16:17.301536
174	174	1	bookarch/00003/Ukhryab.zip	12730	2b6955c37da190450073047fe981d7697190b6aa1b4ccec1744e017909052e4c	\N	\N	f	f	t	f	t	\N	\N	2026-07-10 17:16:17.307023	2026-07-10 17:16:17.307023
175	175	1	bookarch/00003/Fokus-gruppa__Sbornik.zip	217556	8780326e7ab2f74839d3ed9b7e4352c320e34754023e8aac5bdeb21d8d0d8a35	\N	\N	f	f	t	f	t	\N	\N	2026-07-10 17:16:17.337222	2026-07-10 17:16:17.337222
176	176	1	bookarch/00003/Khrustalnyy_mir.zip	25425	18773085e830725f901d1e2debb62663e22d4b48df7b91f2d77c17055ab0264e	\N	\N	f	f	t	f	t	\N	\N	2026-07-10 17:16:17.345142	2026-07-10 17:16:17.345142
177	177	1	bookarch/00003/Chapaev_i_Pustota.zip	323003	be18db76107d3300603615a2ae5443adfda325b7b80e4e63c0fd7b88fb167f32	\N	\N	f	f	t	f	t	\N	\N	2026-07-10 17:16:17.38856	2026-07-10 17:16:17.38856
178	178	1	bookarch/00003/Chisla.zip	210023	943f00d302a568025d15e954a9b3f6f0f597511564dd1eaee73ae9e7efe2fabb	\N	\N	f	f	t	f	t	\N	\N	2026-07-10 17:16:17.406061	2026-07-10 17:16:17.406061
179	179	1	bookarch/00003/Shlem_uzhasa.zip	135562	5ef700bc782d0144181e122dedc745341261e6d6c67f05337e4f9642ebfc82f5	\N	\N	f	f	t	f	t	\N	\N	2026-07-10 17:16:17.42286	2026-07-10 17:16:17.42286
180	180	1	bookarch/00003/Esse__stati.zip	307683	01076c14c7209261cc354155e931373cef1ef0807a1e1f85c2e01bab6b8abfed	\N	\N	f	f	t	f	t	\N	\N	2026-07-10 17:16:17.443056	2026-07-10 17:16:17.443056
181	181	1	bookarch/00003/Smotritel__Tom_1__Orden_zheltogo_flaga.zip	1835643	99de1843a7863f36debdb4f0827e6a96ff68809537a710b23dc9dca84be70b26	\N	\N	f	f	t	f	t	\N	\N	2026-07-10 17:16:17.507905	2026-07-10 17:16:17.507905
182	182	1	bookarch/00003/Smotritel__Kniga_2__Zheleznaya_bezdna.zip	1517548	d1271422a9ec43cf76360743552176fd219b10d52fff2b13c6e2cdb914b988e4	\N	\N	f	f	t	f	t	\N	\N	2026-07-10 17:16:17.566502	2026-07-10 17:16:17.566502
183	183	6	bookarch/00003/Пелевин В. - Круть (Трансгуманизм - 4) - 2024.a4.zip	1634777	b2f5906c4d09b0f2bd28f62366b8ab7d1e182da04222aadc2ff40377cb65b6ca	\N	\N	f	f	t	f	t	\N	\N	2026-07-10 17:16:17.606896	2026-07-10 17:16:17.606896
184	184	1	bookarch/00003/Krut.zip	1132831	0a91ebe029c1eee67bd871907861ece68c26d4e737370e594def4af737f55e54	\N	\N	f	f	t	f	t	\N	\N	2026-07-10 17:16:17.664933	2026-07-10 17:16:17.664933
185	185	1	bookarch/00003/Apologiya_Sokrata.zip	51173	0797c615b35d1f8e4abd64cf6ff4fbf48cb0bcb824174b5db21e05a9ca1ed54f	\N	\N	f	f	t	f	t	\N	\N	2026-07-10 17:16:17.677118	2026-07-10 17:16:17.677118
186	186	1	bookarch/00003/Dialogi.zip	2489309	d714d6610ef0b99eb203514c25b21710bd36dd8afec7adf69a049da475b83859	\N	\N	f	f	t	f	t	\N	\N	2026-07-10 17:16:18.012986	2026-07-10 17:16:18.012986
187	187	1	bookarch/00003/Pelevin_i_pokolenie_pustoty.zip	893247	01856899d98f99571bebb15d8df070e4349dbb258d6e0566c40eb8a3c0d9dc61	\N	\N	f	f	t	f	t	\N	\N	2026-07-10 17:16:18.055594	2026-07-10 17:16:18.055594
188	188	8	bookarch/00003/Psikhologiya_vliyaniya.zip	886419	1afb7dc4592d22e2b40bbfd457fe879a57fdce078733dff644627072c74ca97a	\N	\N	f	f	t	f	t	\N	\N	2026-07-10 17:16:50.755504	2026-07-10 17:16:50.755504
189	189	1	bookarch/00003/Svoboda_Shamana.zip	132367	289e871a9b7675d29f75d0e37c9a556408e8ebe0670416a0a26655f4d6691b84	\N	\N	f	f	t	f	t	\N	\N	2026-07-10 17:16:50.771508	2026-07-10 17:16:50.771508
190	190	1	bookarch/00003/Khokhot_shamana.zip	119942	3a897517f2be1fc87d638d0951960197b6dccbe1e9eb2b779ab514fba0663662	\N	\N	f	f	t	f	t	\N	\N	2026-07-10 17:16:50.785944	2026-07-10 17:16:50.785944
191	191	1	bookarch/00003/Shamanskiy_Les.zip	152284	5df0ab54c622fe60c887108d2c4d003c45327f6e97203c83abf58d6fb1055b11	\N	\N	f	f	t	f	t	\N	\N	2026-07-10 17:16:50.803971	2026-07-10 17:16:50.803971
192	192	1	bookarch/00003/Budushchaya_zapreshchennaya_kniga.zip	7196	46be094c835551d5817f67e2ec5c1407630c7bac973981c09dcdd294c4d66315	\N	\N	f	f	t	f	t	\N	\N	2026-07-10 17:16:50.80986	2026-07-10 17:16:50.80986
193	193	1	bookarch/00003/Viktor_Pelevin__evolyutsiya_v_postmodernizme.zip	27936	9fdf6306d81b9dbd92e966784ff9d8bd698d0436d3abc0a2ced4ee9542e9d77b	\N	\N	f	f	t	f	t	\N	\N	2026-07-10 17:16:50.819177	2026-07-10 17:16:50.819177
194	194	3	bookarch/00003/Kak_upravlyat_rabami.zip	810307	db4d70a9933eedfd771827e2992005ce9e8b5b2b3e39eb2ea40235eb7c99fd76	\N	\N	f	f	t	f	t	\N	\N	2026-07-10 17:16:50.8385	2026-07-10 17:16:50.8385
195	195	6	bookarch/00003/Chyornyy_lebed__Pod_znakom_nepredskazuemosti.zip	1509372	1ade2e4d2b85eda3dfe3e3afb063c68d6301eb08f2c8f15eeae25f3efe41b081	\N	\N	f	f	t	f	t	\N	\N	2026-07-10 17:17:30.231262	2026-07-10 17:17:30.231262
196	196	6	bookarch/00003/Chetvertaya_promyshlennaya_revolyutsiya___Top_Business_Awards.zip	1427995	b64c0272bf57c16d884201c37a70811a889e65f2cc1cc2af2b9d8f75afe5e6b4	\N	\N	f	f	t	f	t	\N	\N	2026-07-10 17:18:05.551176	2026-07-10 17:18:05.551176
197	197	1	bookarch/00003/Dnevnik_pisatelya.zip	700523	a23554638136eb35d2884b81ca7a9e477fd044f02aadb8ca56e4f917ec7f441c	\N	\N	f	f	t	f	t	\N	\N	2026-07-10 17:18:05.645105	2026-07-10 17:18:05.645105
198	198	6	bookarch/00003/косметическая химия.zip	16380725	21069e53c15adee36931f6adaac9ca08c184f9c5ec82d9093296242f89aa7974	\N	\N	f	f	t	f	t	\N	\N	2026-07-10 17:18:06.053345	2026-07-10 17:18:06.053345
199	199	6	bookarch/00003/Matematicheskaya_statistika.zip	1802491	3bd2b52870a4c47ccb3f0b793cfeb7d0f17af619093ca011bdb7737d15344ef2	\N	\N	f	f	t	f	t	\N	\N	2026-07-10 17:18:42.731667	2026-07-10 17:18:42.731667
200	200	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-12 12:18:02.496071	2026-07-12 12:18:02.496071
201	201	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-12 12:18:02.503149	2026-07-12 12:18:02.503149
202	202	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-12 12:18:02.515928	2026-07-12 12:18:02.515928
204	204	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-12 12:18:02.548976	2026-07-12 12:18:02.548976
205	205	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-12 12:18:02.573093	2026-07-12 12:18:02.573093
206	206	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-12 12:18:02.591618	2026-07-12 12:18:02.591618
207	207	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-12 12:18:02.606438	2026-07-12 12:18:02.606438
208	208	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-12 12:18:02.612695	2026-07-12 12:18:02.612695
209	209	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-12 12:18:02.624208	2026-07-12 12:18:02.624208
210	210	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-12 12:18:02.644655	2026-07-12 12:18:02.644655
211	211	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-12 12:18:02.663862	2026-07-12 12:18:02.663862
212	212	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-12 12:18:02.997813	2026-07-12 12:18:02.997813
213	213	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-12 12:18:03.014685	2026-07-12 12:18:03.014685
215	215	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-12 12:18:03.047847	2026-07-12 12:18:03.047847
216	216	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-12 12:18:03.06264	2026-07-12 12:18:03.06264
217	217	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-12 12:18:32.256662	2026-07-12 12:18:32.256662
218	218	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-12 12:18:32.262755	2026-07-12 12:18:32.262755
219	219	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-12 12:18:32.272855	2026-07-12 12:18:32.272855
221	221	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-12 12:18:32.308558	2026-07-12 12:18:32.308558
222	222	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-12 12:18:32.33166	2026-07-12 12:18:32.33166
223	223	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-12 12:18:32.352074	2026-07-12 12:18:32.352074
224	224	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-12 12:18:32.369151	2026-07-12 12:18:32.369151
225	225	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-12 12:18:32.375111	2026-07-12 12:18:32.375111
226	226	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-12 12:18:32.386993	2026-07-12 12:18:32.386993
227	227	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-12 12:18:32.404918	2026-07-12 12:18:32.404918
228	228	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-12 12:18:32.421379	2026-07-12 12:18:32.421379
229	229	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-12 12:18:32.756463	2026-07-12 12:18:32.756463
230	230	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-12 12:18:32.775359	2026-07-12 12:18:32.775359
231	231	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-12 12:18:32.796392	2026-07-12 12:18:32.796392
232	232	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-12 12:18:32.811545	2026-07-12 12:18:32.811545
233	233	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-12 12:18:32.826986	2026-07-12 12:18:32.826986
234	234	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-12 20:10:38.902451	2026-07-12 20:10:38.902451
235	235	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-12 20:10:38.909258	2026-07-12 20:10:38.909258
236	236	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-12 20:10:38.919663	2026-07-12 20:10:38.919663
238	238	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-12 20:10:38.958668	2026-07-12 20:10:38.958668
239	239	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-12 20:10:38.980271	2026-07-12 20:10:38.980271
240	240	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-12 20:10:38.99781	2026-07-12 20:10:38.99781
241	241	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-12 20:10:39.015863	2026-07-12 20:10:39.015863
242	242	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-12 20:10:39.021881	2026-07-12 20:10:39.021881
243	243	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-12 20:10:39.034065	2026-07-12 20:10:39.034065
244	244	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-12 20:10:39.049522	2026-07-12 20:10:39.049522
245	245	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-12 20:10:39.064685	2026-07-12 20:10:39.064685
246	246	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-12 20:10:39.366897	2026-07-12 20:10:39.366897
247	247	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-12 20:10:39.383509	2026-07-12 20:10:39.383509
248	248	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-12 20:10:39.399412	2026-07-12 20:10:39.399412
249	249	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-12 20:10:39.4153	2026-07-12 20:10:39.4153
250	250	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-12 20:10:39.429535	2026-07-12 20:10:39.429535
251	251	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-15 08:44:20.822797	2026-07-15 08:44:20.822797
252	252	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-15 08:44:20.829151	2026-07-15 08:44:20.829151
253	253	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-15 08:44:20.850686	2026-07-15 08:44:20.850686
255	255	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-15 08:44:20.894197	2026-07-15 08:44:20.894197
256	256	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-15 08:44:20.914861	2026-07-15 08:44:20.914861
257	257	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-15 08:44:20.932181	2026-07-15 08:44:20.932181
258	258	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-15 08:44:20.947464	2026-07-15 08:44:20.947464
259	259	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-15 08:44:20.953044	2026-07-15 08:44:20.953044
260	260	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-15 08:44:20.963106	2026-07-15 08:44:20.963106
261	261	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-15 08:44:20.979927	2026-07-15 08:44:20.979927
262	262	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-15 08:44:20.996975	2026-07-15 08:44:20.996975
263	263	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-15 08:44:21.341228	2026-07-15 08:44:21.341228
264	264	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-15 08:44:21.357192	2026-07-15 08:44:21.357192
265	265	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-15 08:44:21.373614	2026-07-15 08:44:21.373614
266	266	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-15 08:44:21.390009	2026-07-15 08:44:21.390009
267	267	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-15 08:44:21.40457	2026-07-15 08:44:21.40457
268	268	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-15 11:10:54.455944	2026-07-15 11:10:54.455944
269	269	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-15 11:10:54.462577	2026-07-15 11:10:54.462577
270	270	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-15 11:10:54.473337	2026-07-15 11:10:54.473337
272	272	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-15 11:10:54.502927	2026-07-15 11:10:54.502927
273	273	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-15 11:10:54.522383	2026-07-15 11:10:54.522383
274	274	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-15 11:10:54.53754	2026-07-15 11:10:54.53754
275	275	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-15 11:10:54.552356	2026-07-15 11:10:54.552356
276	276	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-15 11:10:54.558517	2026-07-15 11:10:54.558517
277	277	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-15 11:10:54.571237	2026-07-15 11:10:54.571237
278	278	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-15 11:10:54.58866	2026-07-15 11:10:54.58866
279	279	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-15 11:10:54.604642	2026-07-15 11:10:54.604642
280	280	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-15 11:10:54.892667	2026-07-15 11:10:54.892667
281	281	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-15 11:10:54.907261	2026-07-15 11:10:54.907261
282	282	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-15 11:10:54.925318	2026-07-15 11:10:54.925318
283	283	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-15 11:10:54.939409	2026-07-15 11:10:54.939409
284	284	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-15 11:10:54.953817	2026-07-15 11:10:54.953817
285	285	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-15 11:17:16.059925	2026-07-15 11:17:16.059925
286	286	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-15 11:17:16.06657	2026-07-15 11:17:16.06657
287	287	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-15 11:17:16.075633	2026-07-15 11:17:16.075633
289	289	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-15 11:17:16.107962	2026-07-15 11:17:16.107962
290	290	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-15 11:17:16.13052	2026-07-15 11:17:16.13052
291	291	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-15 11:17:16.14727	2026-07-15 11:17:16.14727
292	292	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-15 11:17:16.161643	2026-07-15 11:17:16.161643
293	293	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-15 11:17:16.167357	2026-07-15 11:17:16.167357
294	294	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-15 11:17:16.18005	2026-07-15 11:17:16.18005
295	295	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-15 11:17:16.195037	2026-07-15 11:17:16.195037
296	296	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-15 11:17:16.209241	2026-07-15 11:17:16.209241
297	297	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-15 11:17:16.483495	2026-07-15 11:17:16.483495
298	298	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-15 11:17:16.498616	2026-07-15 11:17:16.498616
299	299	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-15 11:17:16.51308	2026-07-15 11:17:16.51308
300	300	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-15 11:17:16.526649	2026-07-15 11:17:16.526649
301	301	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-15 11:17:16.543109	2026-07-15 11:17:16.543109
302	302	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-15 11:25:56.943335	2026-07-15 11:25:56.943335
303	303	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-15 11:25:56.949832	2026-07-15 11:25:56.949832
304	304	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-15 11:25:56.960728	2026-07-15 11:25:56.960728
306	306	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-15 11:25:56.993289	2026-07-15 11:25:56.993289
307	307	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-15 11:25:57.013661	2026-07-15 11:25:57.013661
308	308	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-15 11:25:57.03051	2026-07-15 11:25:57.03051
309	309	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-15 11:25:57.046539	2026-07-15 11:25:57.046539
310	310	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-15 11:25:57.052333	2026-07-15 11:25:57.052333
311	311	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-15 11:25:57.062967	2026-07-15 11:25:57.062967
312	312	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-15 11:25:57.080505	2026-07-15 11:25:57.080505
313	313	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-15 11:25:57.095274	2026-07-15 11:25:57.095274
314	314	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-15 11:26:02.404508	2026-07-15 11:26:02.404508
315	315	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-15 11:26:02.419639	2026-07-15 11:26:02.419639
316	316	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-15 11:26:02.435509	2026-07-15 11:26:02.435509
317	317	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-15 11:26:02.451861	2026-07-15 11:26:02.451861
319	319	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-15 12:31:47.060228	2026-07-15 12:31:47.060228
320	320	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-15 12:31:47.066953	2026-07-15 12:31:47.066953
321	321	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-15 12:31:47.077078	2026-07-15 12:31:47.077078
323	323	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-15 12:31:47.110753	2026-07-15 12:31:47.110753
324	324	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-15 12:31:47.135188	2026-07-15 12:31:47.135188
325	325	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-15 12:31:47.150268	2026-07-15 12:31:47.150268
326	326	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-15 12:31:47.17389	2026-07-15 12:31:47.17389
327	327	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-15 12:31:47.179814	2026-07-15 12:31:47.179814
328	328	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-15 12:31:47.191611	2026-07-15 12:31:47.191611
329	329	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-15 12:31:47.207282	2026-07-15 12:31:47.207282
330	330	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-15 12:31:47.222	2026-07-15 12:31:47.222
331	331	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-15 12:31:47.523246	2026-07-15 12:31:47.523246
332	332	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-15 12:31:47.541337	2026-07-15 12:31:47.541337
333	333	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-15 12:31:47.558714	2026-07-15 12:31:47.558714
334	334	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-15 12:31:47.575156	2026-07-15 12:31:47.575156
335	335	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-15 12:31:47.59113	2026-07-15 12:31:47.59113
336	336	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-15 12:33:20.39546	2026-07-15 12:33:20.39546
337	337	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-15 12:33:20.40135	2026-07-15 12:33:20.40135
338	338	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-15 12:33:20.412097	2026-07-15 12:33:20.412097
340	340	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-15 12:33:20.444583	2026-07-15 12:33:20.444583
341	341	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-15 12:33:20.666521	2026-07-15 12:33:20.666521
342	342	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-15 12:33:20.683895	2026-07-15 12:33:20.683895
343	343	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-15 12:33:20.701301	2026-07-15 12:33:20.701301
344	344	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-15 12:33:20.707766	2026-07-15 12:33:20.707766
345	345	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-15 12:33:20.720266	2026-07-15 12:33:20.720266
346	346	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-15 12:33:20.737465	2026-07-15 12:33:20.737465
347	347	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-15 12:33:20.752633	2026-07-15 12:33:20.752633
348	348	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-15 12:33:21.100618	2026-07-15 12:33:21.100618
349	349	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-15 12:33:21.11749	2026-07-15 12:33:21.11749
350	350	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-15 12:33:21.13349	2026-07-15 12:33:21.13349
351	351	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-15 12:33:21.147393	2026-07-15 12:33:21.147393
353	353	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-15 12:38:23.826639	2026-07-15 12:38:23.826639
354	354	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-15 12:38:23.833383	2026-07-15 12:38:23.833383
355	355	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-15 12:38:23.845596	2026-07-15 12:38:23.845596
357	357	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-15 12:38:23.881484	2026-07-15 12:38:23.881484
358	358	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-15 12:38:23.905436	2026-07-15 12:38:23.905436
359	359	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-15 12:38:23.921829	2026-07-15 12:38:23.921829
360	360	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-15 12:38:23.942764	2026-07-15 12:38:23.942764
361	361	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-15 12:38:23.949812	2026-07-15 12:38:23.949812
362	362	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-15 12:38:23.961966	2026-07-15 12:38:23.961966
363	363	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-15 12:38:23.982726	2026-07-15 12:38:23.982726
364	364	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-15 12:38:23.9982	2026-07-15 12:38:23.9982
365	365	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-15 12:38:24.325114	2026-07-15 12:38:24.325114
366	366	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-15 12:38:24.341511	2026-07-15 12:38:24.341511
367	367	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-15 12:38:24.378098	2026-07-15 12:38:24.378098
368	368	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-15 12:38:24.395247	2026-07-15 12:38:24.395247
369	369	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-15 12:38:24.409845	2026-07-15 12:38:24.409845
370	370	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-15 13:22:41.042904	2026-07-15 13:22:41.042904
371	371	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-15 13:22:41.048772	2026-07-15 13:22:41.048772
372	372	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-15 13:22:41.05998	2026-07-15 13:22:41.05998
374	374	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-15 13:22:41.094024	2026-07-15 13:22:41.094024
375	375	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-15 13:22:41.115042	2026-07-15 13:22:41.115042
376	376	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-15 13:22:41.131793	2026-07-15 13:22:41.131793
377	377	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-15 13:22:41.147237	2026-07-15 13:22:41.147237
378	378	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-15 13:22:41.153154	2026-07-15 13:22:41.153154
379	379	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-15 13:22:41.164743	2026-07-15 13:22:41.164743
380	380	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-15 13:22:41.181428	2026-07-15 13:22:41.181428
381	381	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-15 13:22:41.202014	2026-07-15 13:22:41.202014
382	382	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-15 13:22:41.525658	2026-07-15 13:22:41.525658
383	383	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-15 13:22:41.542953	2026-07-15 13:22:41.542953
384	384	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-15 13:22:41.559167	2026-07-15 13:22:41.559167
385	385	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-15 13:22:41.574774	2026-07-15 13:22:41.574774
387	387	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-15 14:50:32.622829	2026-07-15 14:50:32.622829
388	388	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-15 14:50:32.629095	2026-07-15 14:50:32.629095
389	389	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-15 14:50:32.639384	2026-07-15 14:50:32.639384
391	391	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-15 14:50:32.675058	2026-07-15 14:50:32.675058
392	392	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-15 14:50:32.697854	2026-07-15 14:50:32.697854
393	393	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-15 14:50:32.716891	2026-07-15 14:50:32.716891
394	394	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-15 14:50:32.733255	2026-07-15 14:50:32.733255
395	395	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-15 14:50:32.738559	2026-07-15 14:50:32.738559
396	396	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-15 14:50:32.751238	2026-07-15 14:50:32.751238
397	397	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-15 14:50:32.766676	2026-07-15 14:50:32.766676
398	398	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-15 14:50:32.782569	2026-07-15 14:50:32.782569
399	399	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-15 14:50:33.095301	2026-07-15 14:50:33.095301
400	400	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-15 14:50:33.11146	2026-07-15 14:50:33.11146
401	401	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-15 14:50:33.12665	2026-07-15 14:50:33.12665
402	402	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-15 14:50:33.145271	2026-07-15 14:50:33.145271
403	403	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-15 14:50:33.159857	2026-07-15 14:50:33.159857
404	404	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-15 14:57:25.212397	2026-07-15 14:57:25.212397
405	405	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-15 14:57:25.218584	2026-07-15 14:57:25.218584
406	406	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-15 14:57:25.230155	2026-07-15 14:57:25.230155
408	408	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-15 14:57:25.263708	2026-07-15 14:57:25.263708
409	409	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-15 14:57:25.287246	2026-07-15 14:57:25.287246
410	410	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-15 14:57:25.305507	2026-07-15 14:57:25.305507
411	411	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-15 14:57:25.321305	2026-07-15 14:57:25.321305
412	412	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-15 14:57:25.32718	2026-07-15 14:57:25.32718
413	413	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-15 14:57:25.340215	2026-07-15 14:57:25.340215
414	414	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-15 14:57:25.356639	2026-07-15 14:57:25.356639
415	415	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-15 14:57:25.372583	2026-07-15 14:57:25.372583
416	416	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-15 14:57:25.692501	2026-07-15 14:57:25.692501
417	417	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-15 14:57:25.709233	2026-07-15 14:57:25.709233
418	418	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-15 14:57:25.725789	2026-07-15 14:57:25.725789
419	419	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-15 14:57:25.741801	2026-07-15 14:57:25.741801
421	421	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-15 15:01:46.273532	2026-07-15 15:01:46.273532
422	422	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-15 15:01:46.279582	2026-07-15 15:01:46.279582
423	423	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-15 15:01:46.290162	2026-07-15 15:01:46.290162
425	425	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-15 15:01:46.326025	2026-07-15 15:01:46.326025
426	426	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-15 15:01:46.348715	2026-07-15 15:01:46.348715
427	427	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-15 15:01:46.366337	2026-07-15 15:01:46.366337
428	428	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-15 15:01:46.38426	2026-07-15 15:01:46.38426
429	429	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-15 15:01:46.389854	2026-07-15 15:01:46.389854
430	430	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-15 15:01:46.412626	2026-07-15 15:01:46.412626
431	431	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-15 15:01:46.432084	2026-07-15 15:01:46.432084
432	432	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-15 15:01:46.450281	2026-07-15 15:01:46.450281
433	433	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-15 15:01:46.882943	2026-07-15 15:01:46.882943
434	434	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-15 15:01:46.899738	2026-07-15 15:01:46.899738
435	435	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-15 15:01:46.9163	2026-07-15 15:01:46.9163
436	436	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-15 15:01:46.931549	2026-07-15 15:01:46.931549
437	437	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-15 15:01:46.946302	2026-07-15 15:01:46.946302
438	438	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-15 15:44:42.120611	2026-07-15 15:44:42.120611
439	439	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-15 15:44:42.126907	2026-07-15 15:44:42.126907
440	440	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-15 15:44:42.137584	2026-07-15 15:44:42.137584
442	442	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-15 15:44:42.170426	2026-07-15 15:44:42.170426
443	443	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-15 15:44:42.194225	2026-07-15 15:44:42.194225
444	444	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-15 15:44:42.210132	2026-07-15 15:44:42.210132
445	445	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-15 15:44:42.224617	2026-07-15 15:44:42.224617
446	446	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-15 15:44:42.230332	2026-07-15 15:44:42.230332
447	447	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-15 15:44:42.241793	2026-07-15 15:44:42.241793
448	448	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-15 15:44:42.259819	2026-07-15 15:44:42.259819
449	449	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-15 15:44:42.275485	2026-07-15 15:44:42.275485
450	450	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-15 15:44:42.699648	2026-07-15 15:44:42.699648
451	451	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-15 15:44:42.715546	2026-07-15 15:44:42.715546
452	452	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-15 15:44:42.733712	2026-07-15 15:44:42.733712
453	453	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-15 15:44:42.749068	2026-07-15 15:44:42.749068
454	454	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-15 15:44:42.763748	2026-07-15 15:44:42.763748
455	455	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-16 00:00:18.325954	2026-07-16 00:00:18.325954
456	456	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-16 00:00:18.331879	2026-07-16 00:00:18.331879
457	457	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-16 00:00:18.342737	2026-07-16 00:00:18.342737
459	459	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-16 00:00:18.375011	2026-07-16 00:00:18.375011
460	460	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-16 00:00:18.397376	2026-07-16 00:00:18.397376
461	461	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-16 00:00:18.415814	2026-07-16 00:00:18.415814
462	462	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-16 00:00:18.430578	2026-07-16 00:00:18.430578
463	463	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-16 00:00:18.436432	2026-07-16 00:00:18.436432
464	464	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-16 00:00:18.452206	2026-07-16 00:00:18.452206
465	465	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-16 00:00:18.469925	2026-07-16 00:00:18.469925
466	466	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-16 00:00:18.486004	2026-07-16 00:00:18.486004
467	467	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-16 00:00:18.91478	2026-07-16 00:00:18.91478
468	468	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-16 00:00:18.931826	2026-07-16 00:00:18.931826
469	469	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-16 00:00:18.947752	2026-07-16 00:00:18.947752
470	470	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-16 00:00:18.962742	2026-07-16 00:00:18.962742
471	471	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-16 00:00:18.979901	2026-07-16 00:00:18.979901
472	472	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-16 05:13:10.193857	2026-07-16 05:13:10.193857
473	473	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-16 05:13:10.199859	2026-07-16 05:13:10.199859
474	474	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-16 05:13:10.215997	2026-07-16 05:13:10.215997
476	476	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-16 05:13:10.27087	2026-07-16 05:13:10.27087
477	477	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-16 05:13:10.297237	2026-07-16 05:13:10.297237
478	478	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-16 05:13:10.316674	2026-07-16 05:13:10.316674
479	479	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-16 05:13:10.340774	2026-07-16 05:13:10.340774
480	480	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-16 05:13:10.346708	2026-07-16 05:13:10.346708
481	481	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-16 05:13:10.365052	2026-07-16 05:13:10.365052
482	482	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-16 05:13:10.385207	2026-07-16 05:13:10.385207
483	483	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-16 05:13:10.403912	2026-07-16 05:13:10.403912
484	484	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-16 05:13:11.005959	2026-07-16 05:13:11.005959
485	485	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-16 05:13:11.024593	2026-07-16 05:13:11.024593
486	486	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-16 05:13:11.04492	2026-07-16 05:13:11.04492
487	487	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-16 05:13:11.063093	2026-07-16 05:13:11.063093
488	488	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-16 05:13:11.081085	2026-07-16 05:13:11.081085
489	489	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-16 05:19:07.84064	2026-07-16 05:19:07.84064
490	490	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-16 05:19:07.847173	2026-07-16 05:19:07.847173
491	491	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-16 05:19:07.861986	2026-07-16 05:19:07.861986
493	493	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-16 05:19:07.901903	2026-07-16 05:19:07.901903
494	494	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-16 05:19:07.926913	2026-07-16 05:19:07.926913
495	495	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-16 05:19:07.948587	2026-07-16 05:19:07.948587
496	496	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-16 05:19:07.967495	2026-07-16 05:19:07.967495
497	497	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-16 05:19:07.973958	2026-07-16 05:19:07.973958
498	498	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-16 05:19:07.991001	2026-07-16 05:19:07.991001
499	499	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-16 05:19:08.012593	2026-07-16 05:19:08.012593
500	500	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-16 05:19:08.031522	2026-07-16 05:19:08.031522
501	501	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-16 05:19:08.649389	2026-07-16 05:19:08.649389
502	502	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-16 05:19:08.668586	2026-07-16 05:19:08.668586
503	503	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-16 05:19:08.689174	2026-07-16 05:19:08.689174
504	504	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-16 05:19:08.709535	2026-07-16 05:19:08.709535
505	505	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-16 05:19:08.728588	2026-07-16 05:19:08.728588
506	506	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-16 09:51:24.257907	2026-07-16 09:51:24.257907
507	507	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-16 09:51:24.263958	2026-07-16 09:51:24.263958
508	508	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-16 09:51:24.276236	2026-07-16 09:51:24.276236
510	510	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-16 09:51:24.312074	2026-07-16 09:51:24.312074
511	511	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-16 09:51:24.334036	2026-07-16 09:51:24.334036
512	512	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-16 09:51:24.351483	2026-07-16 09:51:24.351483
513	513	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-16 09:51:24.367311	2026-07-16 09:51:24.367311
514	514	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-16 09:51:24.373909	2026-07-16 09:51:24.373909
515	515	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-16 09:51:24.3866	2026-07-16 09:51:24.3866
516	516	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-16 09:51:24.403205	2026-07-16 09:51:24.403205
517	517	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-16 09:51:24.41911	2026-07-16 09:51:24.41911
518	518	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-16 09:51:24.836834	2026-07-16 09:51:24.836834
519	519	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-16 09:51:24.853991	2026-07-16 09:51:24.853991
520	520	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-16 09:51:24.870654	2026-07-16 09:51:24.870654
521	521	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-16 09:51:24.885862	2026-07-16 09:51:24.885862
522	522	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-16 09:51:24.900963	2026-07-16 09:51:24.900963
523	523	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-16 13:48:25.641523	2026-07-16 13:48:25.641523
524	524	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-16 13:48:25.647507	2026-07-16 13:48:25.647507
525	525	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-16 13:48:25.659262	2026-07-16 13:48:25.659262
527	527	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-16 13:48:25.691298	2026-07-16 13:48:25.691298
528	528	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-16 13:48:25.713685	2026-07-16 13:48:25.713685
529	529	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-16 13:48:25.732219	2026-07-16 13:48:25.732219
530	530	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-16 13:48:25.748534	2026-07-16 13:48:25.748534
531	531	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-16 13:48:25.754315	2026-07-16 13:48:25.754315
532	532	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-16 13:48:25.765161	2026-07-16 13:48:25.765161
533	533	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-16 13:48:25.781191	2026-07-16 13:48:25.781191
534	534	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-16 13:48:25.796213	2026-07-16 13:48:25.796213
535	535	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-16 13:48:26.214669	2026-07-16 13:48:26.214669
536	536	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-16 13:48:26.230894	2026-07-16 13:48:26.230894
537	537	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-16 13:48:26.246806	2026-07-16 13:48:26.246806
538	538	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-16 13:48:26.261889	2026-07-16 13:48:26.261889
539	539	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-16 13:48:26.276186	2026-07-16 13:48:26.276186
540	540	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-16 14:37:13.639082	2026-07-16 14:37:13.639082
541	541	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-16 14:37:13.645387	2026-07-16 14:37:13.645387
542	542	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-16 14:37:13.656564	2026-07-16 14:37:13.656564
544	544	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-16 14:37:13.688819	2026-07-16 14:37:13.688819
545	545	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-16 14:37:13.722114	2026-07-16 14:37:13.722114
546	546	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-16 14:37:13.741516	2026-07-16 14:37:13.741516
547	547	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-16 14:37:13.757441	2026-07-16 14:37:13.757441
548	548	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-16 14:37:13.763147	2026-07-16 14:37:13.763147
549	549	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-16 14:37:13.774287	2026-07-16 14:37:13.774287
550	550	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-16 14:37:13.790365	2026-07-16 14:37:13.790365
551	551	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-16 14:37:13.816803	2026-07-16 14:37:13.816803
552	552	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-16 14:37:14.240742	2026-07-16 14:37:14.240742
553	553	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-16 14:37:14.257462	2026-07-16 14:37:14.257462
554	554	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-16 14:37:14.272864	2026-07-16 14:37:14.272864
555	555	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-16 14:37:14.290553	2026-07-16 14:37:14.290553
557	557	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-16 14:42:15.225621	2026-07-16 14:42:15.225621
558	558	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-16 14:42:15.232175	2026-07-16 14:42:15.232175
559	559	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-16 14:42:15.245018	2026-07-16 14:42:15.245018
561	561	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-16 14:42:15.279652	2026-07-16 14:42:15.279652
562	562	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-16 14:42:15.305864	2026-07-16 14:42:15.305864
563	563	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-16 14:42:15.324802	2026-07-16 14:42:15.324802
564	564	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-16 14:42:15.339339	2026-07-16 14:42:15.339339
565	565	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-16 14:42:15.345205	2026-07-16 14:42:15.345205
566	566	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-16 14:42:15.35659	2026-07-16 14:42:15.35659
567	567	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-16 14:42:15.372357	2026-07-16 14:42:15.372357
568	568	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-16 14:42:15.388905	2026-07-16 14:42:15.388905
569	569	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-16 14:42:15.845192	2026-07-16 14:42:15.845192
570	570	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-16 14:42:15.864239	2026-07-16 14:42:15.864239
571	571	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-16 14:42:15.87988	2026-07-16 14:42:15.87988
572	572	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-16 14:42:15.895821	2026-07-16 14:42:15.895821
573	573	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-16 14:42:15.911775	2026-07-16 14:42:15.911775
574	574	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-16 15:13:29.931365	2026-07-16 15:13:29.931365
575	575	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-16 15:13:29.939036	2026-07-16 15:13:29.939036
576	576	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-16 15:13:29.949942	2026-07-16 15:13:29.949942
578	578	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-16 15:13:29.986276	2026-07-16 15:13:29.986276
579	579	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-16 15:13:30.008294	2026-07-16 15:13:30.008294
580	580	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-16 15:13:30.024446	2026-07-16 15:13:30.024446
581	581	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-16 15:13:30.040315	2026-07-16 15:13:30.040315
582	582	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-16 15:13:30.046638	2026-07-16 15:13:30.046638
583	583	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-16 15:13:30.058745	2026-07-16 15:13:30.058745
584	584	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-16 15:13:30.078098	2026-07-16 15:13:30.078098
585	585	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-16 15:13:30.093933	2026-07-16 15:13:30.093933
586	586	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-16 15:13:30.521409	2026-07-16 15:13:30.521409
587	587	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-16 15:13:30.538622	2026-07-16 15:13:30.538622
588	588	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-16 15:13:30.556039	2026-07-16 15:13:30.556039
589	589	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-16 15:13:30.572756	2026-07-16 15:13:30.572756
590	590	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-16 15:13:30.589395	2026-07-16 15:13:30.589395
591	591	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-16 15:18:54.200887	2026-07-16 15:18:54.200887
592	592	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-16 15:18:54.206965	2026-07-16 15:18:54.206965
593	593	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-16 15:18:54.218422	2026-07-16 15:18:54.218422
595	595	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-16 15:18:54.251429	2026-07-16 15:18:54.251429
596	596	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-16 15:18:54.272473	2026-07-16 15:18:54.272473
597	597	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-16 15:18:54.290027	2026-07-16 15:18:54.290027
598	598	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-16 15:18:54.31107	2026-07-16 15:18:54.31107
599	599	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-16 15:18:54.316896	2026-07-16 15:18:54.316896
600	600	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-16 15:18:54.328862	2026-07-16 15:18:54.328862
601	601	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-16 15:18:54.346092	2026-07-16 15:18:54.346092
602	602	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-16 15:18:54.366598	2026-07-16 15:18:54.366598
603	603	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-16 15:18:54.800548	2026-07-16 15:18:54.800548
604	604	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-16 15:18:54.820132	2026-07-16 15:18:54.820132
605	605	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-16 15:18:54.83711	2026-07-16 15:18:54.83711
606	606	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-16 15:18:54.852558	2026-07-16 15:18:54.852558
608	608	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-16 17:30:24.267049	2026-07-16 17:30:24.267049
609	609	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-16 17:30:24.272829	2026-07-16 17:30:24.272829
610	610	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-16 17:30:24.283475	2026-07-16 17:30:24.283475
612	612	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-16 17:30:24.31887	2026-07-16 17:30:24.31887
613	613	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-16 17:30:24.340765	2026-07-16 17:30:24.340765
614	614	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-16 17:30:24.357448	2026-07-16 17:30:24.357448
615	615	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-16 17:30:24.373031	2026-07-16 17:30:24.373031
616	616	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-16 17:30:24.378939	2026-07-16 17:30:24.378939
617	617	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-16 17:30:24.391772	2026-07-16 17:30:24.391772
618	618	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-16 17:30:24.40963	2026-07-16 17:30:24.40963
619	619	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-16 17:30:24.425615	2026-07-16 17:30:24.425615
620	620	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-16 17:30:24.852068	2026-07-16 17:30:24.852068
621	621	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-16 17:30:24.868011	2026-07-16 17:30:24.868011
622	622	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-16 17:30:24.883889	2026-07-16 17:30:24.883889
623	623	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-16 17:30:24.901372	2026-07-16 17:30:24.901372
624	624	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-16 17:30:24.91708	2026-07-16 17:30:24.91708
625	625	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-16 19:09:11.475866	2026-07-16 19:09:11.475866
626	626	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-16 19:09:11.482977	2026-07-16 19:09:11.482977
627	627	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-16 19:09:11.493664	2026-07-16 19:09:11.493664
629	629	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-16 19:09:11.531303	2026-07-16 19:09:11.531303
630	630	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-16 19:09:11.554085	2026-07-16 19:09:11.554085
631	631	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-16 19:09:11.569106	2026-07-16 19:09:11.569106
632	632	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-16 19:09:11.584764	2026-07-16 19:09:11.584764
633	633	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-16 19:09:11.590444	2026-07-16 19:09:11.590444
634	634	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-16 19:09:11.602721	2026-07-16 19:09:11.602721
635	635	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-16 19:09:11.619578	2026-07-16 19:09:11.619578
636	636	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-16 19:09:11.636018	2026-07-16 19:09:11.636018
637	637	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-16 19:09:12.071822	2026-07-16 19:09:12.071822
638	638	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-16 19:09:12.088964	2026-07-16 19:09:12.088964
639	639	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-16 19:09:12.105906	2026-07-16 19:09:12.105906
640	640	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-16 19:09:12.124156	2026-07-16 19:09:12.124156
641	641	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-16 19:09:12.139744	2026-07-16 19:09:12.139744
642	642	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-17 21:27:15.047921	2026-07-17 21:27:15.047921
643	643	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-17 21:27:15.054215	2026-07-17 21:27:15.054215
644	644	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-17 21:27:15.065208	2026-07-17 21:27:15.065208
646	646	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-17 21:27:15.094445	2026-07-17 21:27:15.094445
647	647	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-17 21:27:15.116195	2026-07-17 21:27:15.116195
648	648	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-17 21:27:15.1349	2026-07-17 21:27:15.1349
649	649	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-17 21:27:15.148364	2026-07-17 21:27:15.148364
650	650	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-17 21:27:15.153879	2026-07-17 21:27:15.153879
651	651	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-17 21:27:15.163861	2026-07-17 21:27:15.163861
652	652	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-17 21:27:15.178931	2026-07-17 21:27:15.178931
653	653	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-17 21:27:15.19212	2026-07-17 21:27:15.19212
654	654	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-17 21:27:15.569617	2026-07-17 21:27:15.569617
655	655	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-17 21:27:15.585243	2026-07-17 21:27:15.585243
656	656	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-17 21:27:15.600539	2026-07-17 21:27:15.600539
657	657	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-17 21:27:15.616273	2026-07-17 21:27:15.616273
658	658	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-17 21:27:15.631743	2026-07-17 21:27:15.631743
659	659	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-18 09:12:30.470262	2026-07-18 09:12:30.470262
660	660	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-18 09:12:30.486615	2026-07-18 09:12:30.486615
661	661	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-18 09:12:30.497268	2026-07-18 09:12:30.497268
663	663	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-18 09:12:30.539482	2026-07-18 09:12:30.539482
664	664	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-18 09:12:30.563162	2026-07-18 09:12:30.563162
665	665	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-18 09:12:30.580551	2026-07-18 09:12:30.580551
666	666	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-18 09:12:30.597838	2026-07-18 09:12:30.597838
667	667	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-18 09:12:30.603254	2026-07-18 09:12:30.603254
668	668	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-18 09:12:30.615992	2026-07-18 09:12:30.615992
669	669	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-18 09:12:30.632468	2026-07-18 09:12:30.632468
670	670	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-18 09:12:30.649945	2026-07-18 09:12:30.649945
671	671	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-18 09:12:31.131251	2026-07-18 09:12:31.131251
672	672	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-18 09:12:31.150943	2026-07-18 09:12:31.150943
673	673	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-18 09:12:31.167339	2026-07-18 09:12:31.167339
674	674	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-18 09:12:31.186479	2026-07-18 09:12:31.186479
676	676	1	bookarch/00003/Parazity_soznaniya.zip	255203	122aeb2861e550fc8295f3fb27213f3e548e51a4680431ece74140683cbbc1f2	\N	\N	f	f	t	f	t	\N	\N	2026-07-20 10:38:30.159804	2026-07-20 10:38:30.159804
677	677	1	bookarch/00003/KRASNAYa_KNIGA.zip	363874	d22e39c0f054ab20202219b2227b0076f9609a942935b09d9d15d715a7068a1f	\N	\N	f	f	t	f	t	\N	\N	2026-07-20 12:09:39.385954	2026-07-20 12:09:39.385954
678	678	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-20 15:01:22.430491	2026-07-20 15:01:22.430491
679	679	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-20 15:01:22.436108	2026-07-20 15:01:22.436108
680	680	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-20 15:01:22.447129	2026-07-20 15:01:22.447129
682	682	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-20 15:01:22.481121	2026-07-20 15:01:22.481121
683	683	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-20 15:01:22.506192	2026-07-20 15:01:22.506192
684	684	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-20 15:01:22.526038	2026-07-20 15:01:22.526038
685	685	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-20 15:01:22.543141	2026-07-20 15:01:22.543141
686	686	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-20 15:01:22.548669	2026-07-20 15:01:22.548669
687	687	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-20 15:01:22.562157	2026-07-20 15:01:22.562157
688	688	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-20 15:01:22.582043	2026-07-20 15:01:22.582043
689	689	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-20 15:01:22.601465	2026-07-20 15:01:22.601465
690	690	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-20 15:01:23.10249	2026-07-20 15:01:23.10249
691	691	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-20 15:01:23.120299	2026-07-20 15:01:23.120299
692	692	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-20 15:01:23.13512	2026-07-20 15:01:23.13512
693	693	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-20 15:01:23.154087	2026-07-20 15:01:23.154087
695	695	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-20 17:32:12.962153	2026-07-20 17:32:12.962153
696	696	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-20 17:32:12.967865	2026-07-20 17:32:12.967865
697	697	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-20 17:32:12.980678	2026-07-20 17:32:12.980678
699	699	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-20 17:32:13.018689	2026-07-20 17:32:13.018689
700	700	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-20 17:32:13.044077	2026-07-20 17:32:13.044077
701	701	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-20 17:32:13.062034	2026-07-20 17:32:13.062034
702	702	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-20 17:32:13.078359	2026-07-20 17:32:13.078359
703	703	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-20 17:32:13.083713	2026-07-20 17:32:13.083713
704	704	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-20 17:32:13.10142	2026-07-20 17:32:13.10142
705	705	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-20 17:32:13.118734	2026-07-20 17:32:13.118734
706	706	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-20 17:32:13.136817	2026-07-20 17:32:13.136817
707	707	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-20 17:32:13.612781	2026-07-20 17:32:13.612781
708	708	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-20 17:32:13.63024	2026-07-20 17:32:13.63024
709	709	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-20 17:32:13.648537	2026-07-20 17:32:13.648537
710	710	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-20 17:32:13.664145	2026-07-20 17:32:13.664145
711	711	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-20 17:32:13.680968	2026-07-20 17:32:13.680968
712	712	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-20 18:45:53.345029	2026-07-20 18:45:53.345029
713	713	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-20 18:45:53.351772	2026-07-20 18:45:53.351772
714	714	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-20 18:45:53.362712	2026-07-20 18:45:53.362712
716	716	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-20 18:45:53.398766	2026-07-20 18:45:53.398766
717	717	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-20 18:45:53.420315	2026-07-20 18:45:53.420315
718	718	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-20 18:45:53.441716	2026-07-20 18:45:53.441716
719	719	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-20 18:45:53.460612	2026-07-20 18:45:53.460612
720	720	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-20 18:45:53.465728	2026-07-20 18:45:53.465728
721	721	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-20 18:45:53.478085	2026-07-20 18:45:53.478085
722	722	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-20 18:45:53.500753	2026-07-20 18:45:53.500753
723	723	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-20 18:45:53.516949	2026-07-20 18:45:53.516949
724	724	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-20 18:45:53.974118	2026-07-20 18:45:53.974118
725	725	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-20 18:45:53.994799	2026-07-20 18:45:53.994799
726	726	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-20 18:45:54.013938	2026-07-20 18:45:54.013938
727	727	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-20 18:45:54.030519	2026-07-20 18:45:54.030519
728	728	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-20 18:45:54.045814	2026-07-20 18:45:54.045814
729	729	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-21 13:08:39.385614	2026-07-21 13:08:39.385614
730	730	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-21 13:08:39.396844	2026-07-21 13:08:39.396844
731	731	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-21 13:08:39.409464	2026-07-21 13:08:39.409464
733	733	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-21 13:08:39.449785	2026-07-21 13:08:39.449785
734	734	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-21 13:08:39.471339	2026-07-21 13:08:39.471339
735	735	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-21 13:08:39.488401	2026-07-21 13:08:39.488401
736	736	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-21 13:08:39.504794	2026-07-21 13:08:39.504794
737	737	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-21 13:08:39.510116	2026-07-21 13:08:39.510116
738	738	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-21 13:08:39.524471	2026-07-21 13:08:39.524471
739	739	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-21 13:08:39.541718	2026-07-21 13:08:39.541718
740	740	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-21 13:08:39.560093	2026-07-21 13:08:39.560093
741	741	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-21 13:08:40.01312	2026-07-21 13:08:40.01312
742	742	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-21 13:08:40.030444	2026-07-21 13:08:40.030444
743	743	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-21 13:08:40.048555	2026-07-21 13:08:40.048555
744	744	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-21 13:08:40.064763	2026-07-21 13:08:40.064763
745	745	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-21 13:08:40.083217	2026-07-21 13:08:40.083217
746	746	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-21 14:59:09.716402	2026-07-21 14:59:09.716402
747	747	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-21 14:59:09.723218	2026-07-21 14:59:09.723218
748	748	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-21 14:59:09.734982	2026-07-21 14:59:09.734982
750	750	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-21 14:59:09.77278	2026-07-21 14:59:09.77278
751	751	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-21 14:59:09.797791	2026-07-21 14:59:09.797791
752	752	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-21 14:59:09.814749	2026-07-21 14:59:09.814749
753	753	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-21 14:59:09.831287	2026-07-21 14:59:09.831287
754	754	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-21 14:59:09.837351	2026-07-21 14:59:09.837351
755	755	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-21 14:59:09.853609	2026-07-21 14:59:09.853609
756	756	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-21 14:59:09.874288	2026-07-21 14:59:09.874288
757	757	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-21 14:59:09.896072	2026-07-21 14:59:09.896072
758	758	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-21 14:59:10.375155	2026-07-21 14:59:10.375155
759	759	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-21 14:59:10.391754	2026-07-21 14:59:10.391754
760	760	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-21 14:59:10.41118	2026-07-21 14:59:10.41118
761	761	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-21 14:59:10.428556	2026-07-21 14:59:10.428556
762	762	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-21 14:59:10.443922	2026-07-21 14:59:10.443922
763	763	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-21 15:40:16.044542	2026-07-21 15:40:16.044542
764	764	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-21 15:40:16.051771	2026-07-21 15:40:16.051771
765	765	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-21 15:40:16.066138	2026-07-21 15:40:16.066138
767	767	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-21 15:40:16.112249	2026-07-21 15:40:16.112249
768	768	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-21 15:40:16.138729	2026-07-21 15:40:16.138729
769	769	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-21 15:40:16.160175	2026-07-21 15:40:16.160175
770	770	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-21 15:40:16.178137	2026-07-21 15:40:16.178137
771	771	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-21 15:40:16.184526	2026-07-21 15:40:16.184526
772	772	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-21 15:40:16.20389	2026-07-21 15:40:16.20389
773	773	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-21 15:40:16.22509	2026-07-21 15:40:16.22509
774	774	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-21 15:40:16.244406	2026-07-21 15:40:16.244406
775	775	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-21 15:40:16.874415	2026-07-21 15:40:16.874415
776	776	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-21 15:40:16.896279	2026-07-21 15:40:16.896279
777	777	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-21 15:40:16.915469	2026-07-21 15:40:16.915469
778	778	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-21 15:40:16.941188	2026-07-21 15:40:16.941188
779	779	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-21 15:40:16.960577	2026-07-21 15:40:16.960577
780	780	1	bookarch/00003/Orgupravlencheskoe_myshlenie__ideologiya__metodologiya__tekhnologiya.zip	1888907	7e8453101a1c25210cba9b6a2de9eff616e376b443a38427323c7014b9fd76fd	\N	\N	f	f	t	f	t	\N	\N	2026-07-21 15:44:10.227431	2026-07-21 15:44:10.227431
781	781	1	bookarch/00003/KGBT___KGBT.zip	537054	8aa8e86fa6b595200d0dbfa5fc1d0aff3b318cce608956d162ca9c72133953cc	\N	\N	f	f	t	f	t	\N	\N	2026-07-21 15:44:10.290163	2026-07-21 15:44:10.290163
782	782	3	bookarch/00003/Besedy_s_Bogom__Neobychnyy_dialog__Kniga_1.zip	377213	febd83d3c264028c198ac7ccd1afb6ccd4d31d9a8f42fced8c3a3eab3c9521ea	\N	\N	f	f	t	f	t	\N	\N	2026-07-21 15:44:10.306207	2026-07-21 15:44:10.306207
783	783	1	bookarch/00003/Zhyoltyy_vozhd.zip	416906	d27696ef998258671d717a8c16ab5872c4eec1dbdef027c24b08caba149d4ff9	\N	\N	f	f	t	f	t	\N	\N	2026-07-21 15:44:10.335852	2026-07-21 15:44:10.335852
784	784	1	bookarch/00003/Okhotniki_za_skalpami.zip	166955	345e83c45f8e76209978a247b2ead2f666b290959afb8c75791397f7d82bfd14	\N	\N	f	f	t	f	t	\N	\N	2026-07-21 15:44:10.366339	2026-07-21 15:44:10.366339
785	785	1	bookarch/00003/Puteshestvie_v_Elevsin.zip	1123085	ae619357aec30e15b9a32f4bd110cad359c1366c519f89bdb947d31e7fc5bc76	\N	\N	f	f	t	f	t	\N	\N	2026-07-21 15:44:10.443562	2026-07-21 15:44:10.443562
786	786	1	bookarch/00003/Fantasticheskiy_almanakh__Zavtra____Vypusk_chetvertyy.zip	1437392	fe7b1e6b3de07656e36a23162ed176ca511c68d3e994e4822d3d2ca8aa331e85	\N	\N	f	f	t	f	t	\N	\N	2026-07-21 15:44:10.52771	2026-07-21 15:44:10.52771
788	788	1	bookarch/00003/TRANSHUMANISM_INC.zip	1036417	a03c12c913884e26fd52402a52f2fa6d0df4f26e2b314a73d84e3ce1238c4a3a	\N	\N	f	f	t	f	t	\N	\N	2026-07-21 15:44:10.680896	2026-07-21 15:44:10.680896
789	789	8	bookarch/00003/Chyornyy_lebed__Pod_znakom_nepredskazuemosti_1.zip	925186	84d6e457d7795bac043b1318e2d3d6d3388216393753cbb86bf60d87eb748c31	\N	\N	f	f	t	f	t	\N	\N	2026-07-21 15:45:10.406087	2026-07-21 15:45:10.406087
790	790	1	bookarch/00003/Piratskiy_ostrov__Molodye_nevolniki.zip	12071683	d39d00bc492fa6d30d0571f7ae82e390bf637622ca9416430d9a39cb7162212b	\N	\N	f	f	t	f	t	\N	\N	2026-07-21 15:45:10.815939	2026-07-21 15:45:10.815939
791	791	1	bookarch/00003/Smertelnyy_vystrel.zip	5461706	55f89887f8646cc7c34197ac6b8c6812473af21cb1e9bf91922ea49e87c0c212	\N	\N	f	f	t	f	t	\N	\N	2026-07-21 15:45:11.044353	2026-07-21 15:45:11.044353
792	792	1	bookarch/00003/Propavshaya_gora.zip	376180	a339c5a029cf4e0571ee006e1f702a7e5babef4037638b981249834a06e461f8	\N	\N	f	f	t	f	t	\N	\N	2026-07-21 15:45:11.063954	2026-07-21 15:45:11.063954
793	793	1	bookarch/00003/Pronzennoe_serdtse_i_drugie_rasskazy.zip	567349	3e9ef44bcfa5b48e3b1524b1a600dd169ca78fb311b69363b4b2bdf8ddef8fb2	\N	\N	f	f	t	f	t	\N	\N	2026-07-21 15:45:11.086615	2026-07-21 15:45:11.086615
794	794	1	bookarch/00003/Vsadnik_bez_golovy__Morskoy_volchonok.zip	2497251	c3c6a86eb2445e67bd5e32f1c5716b83b92b39e77babaebc99b02fcb763de2ed	\N	\N	f	f	t	f	t	\N	\N	2026-07-21 15:45:11.155943	2026-07-21 15:45:11.155943
795	795	6	bookarch/00003/Mya_navykov_vysoko-effektivnykh_lyudey__moshchnye_instrumenty_razvitiya_lichnosti.zip	1147960	d86c8211ed7504a218ab5aa9f0a2ff75d06c494e9081195fb67162f172a3b140	\N	\N	f	f	t	f	t	\N	\N	2026-07-21 15:45:37.997756	2026-07-21 15:45:37.997756
951	951	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-23 07:17:39.613987	2026-07-23 07:17:39.613987
952	952	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-23 07:17:39.63965	2026-07-23 07:17:39.63965
953	953	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-23 07:17:39.658919	2026-07-23 07:17:39.658919
796	796	1	bookarch/00003/Otvazhnaya_okhotnitsa.zip	1644077	a682a5dc3a9def23afff26da1488018951bb39078eaf6a6648ca5b9195dcc82f	\N	\N	f	f	t	f	t	\N	\N	2026-07-21 15:45:38.110103	2026-07-21 15:45:38.110103
797	797	1	bookarch/00003/Okhota_na_leviafana.zip	841788	8fe8b8f23cffbd2e33e0b48f8bebab7f4b06d5754dd9cceb8e43e8c92431f345	\N	\N	f	f	t	f	t	\N	\N	2026-07-21 15:45:38.156859	2026-07-21 15:45:38.156859
798	798	8	bookarch/00003/ISKUSSTVO_SNOVIDENIYa.zip	285310	05f988dc3aa308412c2b0a4918e921aa82f993b944e62eea1541be50cabee0f2	\N	\N	f	f	t	f	t	\N	\N	2026-07-21 15:46:12.559763	2026-07-21 15:46:12.559763
799	799	6	bookarch/00003/Laracasts_Tips_and_Tricks.zip	1142965	eb85a3e30880d9ea39661cd1c45729b419fd53462999058f0d68e9275f7be5ed	\N	\N	f	f	t	f	t	\N	\N	2026-07-21 15:46:29.363865	2026-07-21 15:46:29.363865
800	800	1	bookarch/00003/Iuda_Iskariot.zip	148259	c04f81da200d28c44b93d25baccb39c7b898367277196884c9aaac66a486e565	\N	\N	f	f	t	f	t	\N	\N	2026-07-21 15:46:29.382077	2026-07-21 15:46:29.382077
801	801	1	bookarch/00003/Zhizn_vnutri_puzyrya__Neformalnoe_rukovodstvo_menedzhera_po_vyzhivaniyu_v_investiruemom_proekte.zip	680601	6078de117b83605738b569f9a1485ee5fd79f197466a3bde4036f266e0592f14	\N	\N	f	f	t	f	t	\N	\N	2026-07-21 15:46:29.412558	2026-07-21 15:46:29.412558
802	802	6	bookarch/00003/Data_Science_from_Scratch__First_Principles_with_Python.zip	10182820	99b7360243d1479822fb2285e2a4f882cd8a5a5efd41c582addbc52c75d61791	\N	\N	f	f	t	f	t	\N	\N	2026-07-21 15:46:45.354403	2026-07-21 15:46:45.354403
803	803	1	bookarch/00003/Vremya___dengi.zip	478629	38ac0c21c909ef69d6106f2cd16634ac6f4ed80021982fbc0044cd23f85566fe	\N	\N	f	f	t	f	t	\N	\N	2026-07-21 15:46:45.395766	2026-07-21 15:46:45.395766
804	804	6	bookarch/00003/Git_для_профессионального_программиста.zip	5408909	a71dad61bf32cda531cecde1e6bb0260f68ab81e5d17ad7c0c482599ef80b827	\N	\N	f	f	t	f	t	\N	\N	2026-07-21 15:46:45.536446	2026-07-21 15:46:45.536446
805	805	1	bookarch/00003/Uskorenie__Sovershenstvovanie_metodov_khozyaystvovaniya.zip	289340	4b77cc2a01510b079052fa795d0d69214153b8aae6bc27d09a4daee1e2ee1fef	\N	\N	f	f	t	f	t	\N	\N	2026-07-21 15:46:45.572924	2026-07-21 15:46:45.572924
806	806	8	bookarch/00003/Taynaya_Doktrina__Sintez_nauki__religii_i_filosofii.zip	1209872	13684de4affa5dcb292758919a852dfd477c1c88b2b0be0b7f4b2fcadb7b2855	\N	\N	f	f	t	f	t	\N	\N	2026-07-21 15:47:29.570287	2026-07-21 15:47:29.570287
807	807	8	bookarch/00003/Taynaya_Doktrina__Sintez_nauki__religii_i_filosofii_Tom_II__Antropogenezis.zip	1503036	1efe8448f633997c36062f9d875db1436286813a2212ed5b6f3918735b11b6b2	\N	\N	f	f	t	f	t	\N	\N	2026-07-21 15:48:18.549859	2026-07-21 15:48:18.549859
808	808	8	bookarch/00003/HPB-TD3.DOC.zip	853697	19058e9ab6c54dd83aee0a5ebbdb3c180e6ffe82f5f4ffb4c9e76ba0a8086251	\N	\N	f	f	t	f	t	\N	\N	2026-07-21 15:49:18.673356	2026-07-21 15:49:18.673356
809	809	6	bookarch/00003/Tekst_knigi__predostavlennyy_cherez_vydelenie__Etu_knigu_khorosho_dopolnyayut___yavlyaetsya_ssylkoy_na_drugie_izdaniya_i_ne_soderzhit_informatsii_o_zaglavii_ili_avtore_samoy_osnovnoy_raboty.zip	3280527	61f57ddf4fcba9f49fec88f56d2bf7799a30248767534e16b1551268afcc25ef	\N	\N	f	f	t	f	t	\N	\N	2026-07-21 15:49:38.65411	2026-07-21 15:49:38.65411
810	810	1	bookarch/00004/Tak_govoril_Zaratustra.zip	329129	7251def502ce591d9c6d8074d92c3baf3ef57b0329808c90cce58723cf478627	\N	\N	f	f	t	f	t	\N	\N	2026-07-21 15:49:38.704073	2026-07-21 15:49:38.704073
811	811	9	bookarch/00004/ORGUPRAVLENChESKOE_MYShLENIE__ideologiya__metodologiya__tekhnologiya.zip	736795	eb8eecfa2a120b6edfe358330e4fae3d00e2ee9f951e9d7f2db587be0dffd84d	\N	\N	f	f	t	f	t	\N	\N	2026-07-21 15:50:20.00399	2026-07-21 15:50:20.00399
812	812	1	bookarch/00004/Almaznaya_Sutra.zip	187503	7ba11555c765ad49fefb4527c4a50ba02fc820e56dbdf15eb6e30163517bcff9	\N	\N	f	f	t	f	t	\N	\N	2026-07-21 15:50:20.035534	2026-07-21 15:50:20.035534
813	813	1	bookarch/00004/Antikriziskaya_programma.zip	61214	cedd95dbfda4d9f661b5edb9fa7be4a31f9228163ff589d9eee988e302cf377b	\N	\N	f	f	t	f	t	\N	\N	2026-07-21 15:50:20.050556	2026-07-21 15:50:20.050556
814	814	1	bookarch/00004/Belyy_Lotos.zip	302971	97862bb2c6778bfa785c00973f63c8d1f9f4d35d4f51b74efbf37f0757e475a7	\N	\N	f	f	t	f	t	\N	\N	2026-07-21 15:50:20.093723	2026-07-21 15:50:20.093723
815	815	1	bookarch/00004/Blizost__Doverie_k_sebe_i_drugomu.zip	156697	b1ca60f28353a166de4db6a87937a38bdbed2e509b856ed65e102617d4dba192	\N	\N	f	f	t	f	t	\N	\N	2026-07-21 15:50:20.11649	2026-07-21 15:50:20.11649
816	816	1	bookarch/00004/Budda__Pustota_Serdtsa.zip	213726	d8efb7fc54fb6f93968867640ce1e6ba318ccfd02faecfcb8919f97f7e1bcd53	\N	\N	f	f	t	f	t	\N	\N	2026-07-21 15:50:20.140947	2026-07-21 15:50:20.140947
817	817	1	bookarch/00004/Gorchichnoe_zerno__Kommentarii_k_pyatomu_Evangeliyu_ot_sv__Fomy.zip	454105	9ddf510e61d5df72d763868aa4ebdbb6bd88c3ad06b9f93fc4758f733a60549a	\N	\N	f	f	t	f	t	\N	\N	2026-07-21 15:50:20.202047	2026-07-21 15:50:20.202047
818	818	1	bookarch/00004/Gus_snaruzhi.zip	197605	9af00e6715625d151f498a2590cf5d1f53f121db9789ea6ed493901009d0057d	\N	\N	f	f	t	f	t	\N	\N	2026-07-21 15:50:20.232632	2026-07-21 15:50:20.232632
819	819	1	bookarch/00004/V_poiskakh_Chudesnogo__Chakry__Kundalini_i_sem_tel.zip	295951	0c0f21339202a68c0f2f5bacee17932ff64a13e51b90da745696ab73167d53c8	\N	\N	f	f	t	f	t	\N	\N	2026-07-21 15:50:20.275048	2026-07-21 15:50:20.275048
820	820	1	bookarch/00004/Almaznye_rossypi.zip	50725	8773079fcde6809d754b74107a3e4406a2e991260d9d4d852d659e63a2dfbb45	\N	\N	f	f	t	f	t	\N	\N	2026-07-21 15:50:20.286397	2026-07-21 15:50:20.286397
821	821	8	bookarch/00004/Rukovodstvo_k_svodu_znaniy_po_upravleniyu_proektami__Rukovodstvo_PMBOK___Redaktsiya_2000_goda.zip	773814	6f89cd1e6ec94d05f0d4b0370bf9624b2d9e023b203a6b976a6a9adf57c3026f	\N	\N	f	f	t	f	t	\N	\N	2026-07-21 15:51:00.123321	2026-07-21 15:51:00.123321
822	822	1	bookarch/00004/Lampa_Mafusaila__ili_Kraynyaya_bitva_chekistov_s_masonami.zip	912555	8bfa1ad402573357edc25f8baee00274ebf6a914d0dea0383c86cc9996e0e2d9	\N	\N	f	f	t	f	t	\N	\N	2026-07-21 15:51:00.182008	2026-07-21 15:51:00.182008
823	823	1	bookarch/00004/Iskusstvo_legkikh_kasaniy.zip	1795392	83000a22d1c8ef963e22f720d793e2cb77e792fd48119dd7e640298d0e696fd4	\N	\N	f	f	t	f	t	\N	\N	2026-07-21 15:51:00.258903	2026-07-21 15:51:00.258903
824	824	1	bookarch/00004/Nepobedimoe_solntse.zip	581036	a5e0cf6606e101424cd961fdbf3d52924c24b9998dc84e3224414ee5e861a424	\N	\N	f	f	t	f	t	\N	\N	2026-07-21 15:51:00.320256	2026-07-21 15:51:00.320256
825	825	1	bookarch/00004/iPhuck_10.zip	915814	409ad58a5bc2cbed737c56431f5bc839510ca3ed73081640ebfc94c18acca44e	\N	\N	f	f	t	f	t	\N	\N	2026-07-21 15:51:00.380568	2026-07-21 15:51:00.380568
826	826	1	bookarch/00004/Shum__Nesovershenstvo_chelovecheskikh_suzhdeniy.zip	1023672	034c76fe8c795b5e6875cead3ccc74adb804ab462727044f89ec03e2f461cdb1	\N	\N	f	f	t	f	t	\N	\N	2026-07-21 15:51:00.455477	2026-07-21 15:51:00.455477
827	827	1	bookarch/00004/Anna_Karenina.zip	1568215	bf9ad34d739a152c39d9ef7aa74b98def0e74c8b65c335e8e7cd233cead9a826	\N	\N	f	f	t	f	t	\N	\N	2026-07-21 15:51:00.588068	2026-07-21 15:51:00.588068
828	828	1	bookarch/00004/Vozvrashchenie_Siney_Borody.zip	571562	0f58f01ad29dc6a1ae3c919c903d484356b3bf7df76c2a9f1b8a38e632d2cab9	\N	\N	f	f	t	f	t	\N	\N	2026-07-21 15:51:00.640797	2026-07-21 15:51:00.640797
830	830	8	bookarch/00004/Zhizn-v-snovidenii__Posvyashchenie_v_mir_magov.zip	433516	2613e075f6bb7422cf2fa2835f49a1501e550a3d67da1c34c9b6046314dc5af0	\N	\N	f	f	t	f	t	\N	\N	2026-07-21 15:51:39.683694	2026-07-21 15:51:39.683694
831	831	1	bookarch/00004/Dogovoritsya_mozhno_obo_vsem.zip	317425	910f65b487966a11cdc16be83c1fd035093fcc80b57b8ec682c4bde7f8dd4eb8	\N	\N	f	f	t	f	t	\N	\N	2026-07-21 15:51:39.729419	2026-07-21 15:51:39.729419
832	832	6	bookarch/00004/Investiruy_v_Sebya__Razbudi_v_sebe_ispolina.zip	2616166	fc524cef4e3ec0e4c2822a64ddb78ded8d2257e5bd852df8b4a837cc67c4ea2f	\N	\N	f	f	t	f	t	\N	\N	2026-07-21 15:52:16.550161	2026-07-21 15:52:16.550161
833	833	8	bookarch/00004/Biznes_v_stile_fank__Kapital_plyashet_pod_dudku_talanta__otryvki_iz_knigi.zip	148959	125c7b61dd9da5eb7e04caac132bff03e7ccdb3b7c26dd470ecc22de543f7a73	\N	\N	f	f	t	f	t	\N	\N	2026-07-21 15:53:01.242092	2026-07-21 15:53:01.242092
871	871	1	bookarch/00004/Koleso_vremeni.zip	76033	b74c22864569ea7784ab98d7ba2e2e9b7ef2f52ca3fcdbd40bfcfb9150b97b2b	\N	\N	f	f	t	f	t	\N	\N	2026-07-21 16:07:37.647815	2026-07-21 16:07:37.647815
834	834	8	bookarch/00004/Taynaya_doktrina__Sintez_nauki__religii_i_filosofii__Tom_I__Kosmogenezis.zip	1215052	75c43276141e4de67e4fe3810dd6963d3983b634bf629c69017c21ade039498c	\N	\N	f	f	t	f	t	\N	\N	2026-07-21 15:53:48.025576	2026-07-21 15:53:48.025576
835	835	1	bookarch/00004/iPhuck_10_1.zip	448958	ba02dc1c1e1aec9c697b0dcda037c4db5fa26241877a75f57dfe54ed167ad7e7	\N	\N	f	f	t	f	t	\N	\N	2026-07-21 15:53:48.105012	2026-07-21 15:53:48.105012
836	836	8	bookarch/00004/isis1.zip	873511	bbf555a22170943dd78f0e1b546304cad4985fa152b2a4e40da3e2d5e33f97f6	\N	\N	f	f	t	f	t	\N	\N	2026-07-21 15:54:48.286164	2026-07-21 15:54:48.286164
837	837	8	bookarch/00004/Puteshestvie_k_tsentru_Zemli.zip	1660036	8874fd33a43c048b6b08ff5ce4399f69e1cfc49983aec23d833e546e0d283481	\N	\N	f	f	t	f	t	\N	\N	2026-07-21 15:55:27.376747	2026-07-21 15:55:27.376747
838	838	1	bookarch/00004/Kontekst_zhizni__Kak_nauchitsya_upravlyat_privychkami__kotorye_upravlyayut_nami.zip	233382	8dd86253f5265455d6ab97cad23a1624fcf79df4fa4b9b87dc3e69081c2e6498	\N	\N	f	f	t	f	t	\N	\N	2026-07-21 15:55:27.399981	2026-07-21 15:55:27.399981
839	839	8	bookarch/00004/Magicheskiy_perekhod__Put_zhenshchiny-voina.zip	321603	08f5f9c61b80eed82e22504520f59e9a02ea01042bf82f85b5d2a05bc663f790	\N	\N	f	f	t	f	t	\N	\N	2026-07-21 15:56:12.463689	2026-07-21 15:56:12.463689
840	840	1	bookarch/00004/Ponedelnik_nachinaetsya_v_subbotu.zip	532438	aee30ad9ce87d3eb5aed818e300e69a153c5a21dd6374515e80ac29cc354b8d4	\N	\N	f	f	t	f	t	\N	\N	2026-07-21 15:56:12.504626	2026-07-21 15:56:12.504626
841	841	8	bookarch/00004/SHABONO_A_True_Adventure_in_the_Remote_and_Magical_Heart_of_the_South_American_Jungle.zip	256308	3373a293d504e292c7a958c33dc1237c567b1e8ea3e6b258f277bbdfd8d38030	\N	\N	f	f	t	f	t	\N	\N	2026-07-21 15:57:02.278284	2026-07-21 15:57:02.278284
842	842	8	bookarch/00004/Magicheskie_passy__Prakticheskaya_mudrost_shamanov_drevney_Meksiki.zip	1592464	e2e08f4f00f9b57eecd838ab19cf1ed6f10998081ec790b69ddc02cc38d86c13	\N	\N	f	f	t	f	t	\N	\N	2026-07-21 15:57:42.098163	2026-07-21 15:57:42.098163
843	843	8	bookarch/00004/Put_voina__Dva_goda_s_doktorom_Khuanom_Matusom_-_Son_vedmy.zip	226632	e9cb5cec3438ceb21a93e11fe5ff4ea627aa561682aa04297abf108cf49399f3	\N	\N	f	f	t	f	t	\N	\N	2026-07-21 15:58:23.448196	2026-07-21 15:58:23.448196
844	844	8	bookarch/00004/Vzglyady_iz_realnogo_mira__Zapisi_besed_i_lektsiy_Gurdzhieva__Samonablyudenie.zip	89160	dac1bb8e4362f47ecb11f4fcf68b75f1fd0ea9b58b693a85d0376f593f73c5ef	\N	\N	f	f	t	f	t	\N	\N	2026-07-21 15:59:05.869285	2026-07-21 15:59:05.869285
845	845	1	bookarch/00004/NF__Almanakh_nauchnoy_fantastiki_35__1991.zip	410646	1fefd8f1ca11f3d73fe2ced7907438a62475d1240290605fbea894eb402a3b2a	\N	\N	f	f	t	f	t	\N	\N	2026-07-21 15:59:05.914317	2026-07-21 15:59:05.914317
846	846	8	bookarch/00004/Rasskazy_Velzevula_svoemu_vnuku__Obektivno-bespristrastnaya_kritika_zhizni_lyudey__Vsyo_i_Vsya__v_trekh_seriyakh.zip	1015432	9f1d0a70918f32da43ed8cdd6aac4269d817758a16f404b2fffbfba8cc55746f	\N	\N	f	f	t	f	t	\N	\N	2026-07-21 15:59:47.799375	2026-07-21 15:59:47.799375
847	847	8	bookarch/00004/Shestoe_i_poslednee_prebyvanie_Velzevula_na_poverkhnosti_Nashey_Zemli__chast_iz_tsikla_romanov__Arkhiepiskop_Pletenetskiy.zip	307316	0db74ae90032ff012778a78dfe8dc6c76c942e379fb779b7c75000d8c54c6585	\N	\N	f	f	t	f	t	\N	\N	2026-07-21 16:00:29.791841	2026-07-21 16:00:29.791841
848	848	8	bookarch/00004/Svet_na_Puti_v_Goru_Svyatykh__Velikiy_Uchitel_Gautam_i_Druzya__Kniga_1.zip	827330	ac882563e26cef7d72f862c4ac968e569bc09a70d29d96832b932fe9e7129883	\N	\N	f	f	t	f	t	\N	\N	2026-07-21 16:01:07.795816	2026-07-21 16:01:07.795816
849	849	8	bookarch/00004/Golos_bezmolviya__ili_Dva_puti__Sem_vrat__iz_sokrovennykh_indusskikh_pisaniy.zip	65254	b9e5720167bf3659ce151aa7044ac9d0ddbbe11288ef41d595f8fb50f1da92c9	\N	\N	f	f	t	f	t	\N	\N	2026-07-21 16:02:00.708856	2026-07-21 16:02:00.708856
850	850	1	bookarch/00004/Besedy_s_uchenikami.zip	82208	bfdffb4446dd1971ff32be06cb7a10660436891dc4fb6d61b343402fe90d12ae	\N	\N	f	f	t	f	t	\N	\N	2026-07-21 16:02:00.721419	2026-07-21 16:02:00.721419
851	851	1	bookarch/00004/Vse_i_vsya.zip	958151	29dbd4d62e5347b6758fede0bd3c38aaf422615baeedf6f243ba5ebf23846dc0	\N	\N	f	f	t	f	t	\N	\N	2026-07-21 16:02:00.815672	2026-07-21 16:02:00.815672
852	852	1	bookarch/00004/Vstrechi_s_zamechatelnymi_lyudmi.zip	257866	cff3183b7894ec1fd8c0d0060fa6e838ab6c6facbd26a933d4cf90137a5cc4f0	\N	\N	f	f	t	f	t	\N	\N	2026-07-21 16:02:00.862362	2026-07-21 16:02:00.862362
853	853	1	bookarch/00004/Vstrechi_s_zamechatelnymi_lyudmi_1.zip	257878	6f807fa534bc780b4537999333773c57a9edaccffc1023f86d1ce3045933c23c	\N	\N	f	f	t	f	t	\N	\N	2026-07-21 16:02:00.906156	2026-07-21 16:02:00.906156
854	854	1	bookarch/00004/Zhizn_realna_tolko_togda__kogda__Ya_est.zip	218949	6f9b38eb60aca83fd7807db6bad81700e6076772f7eee406654b5aae9c647578	\N	\N	f	f	t	f	t	\N	\N	2026-07-21 16:02:00.936861	2026-07-21 16:02:00.936861
855	855	1	bookarch/00004/Zakonomernoe_raznoobrazie_proyavleniy_chelovecheskoy_individualnosti.zip	31567	090ecf29653c9272c20742d13b072f509ee64f9f8ffbcd937b29354faa5f4208	\N	\N	f	f	t	f	t	\N	\N	2026-07-21 16:02:00.945292	2026-07-21 16:02:00.945292
856	856	1	bookarch/00004/Posledniy_chas_zhizni.zip	7966	4f78e2868169ca0ad77391df72a3545714bed1d398a8ce150a38e8c10a7ec4d8	\N	\N	f	f	t	f	t	\N	\N	2026-07-21 16:02:00.952545	2026-07-21 16:02:00.952545
857	857	1	bookarch/00004/Chelovek_-_eto_mnogoslozhnoe_sushchestvo.zip	163870	3a8bc8ae09e7fe7996a381d124e0e9f86a0729d84f6ceb2a42ef0cf327cc6442	\N	\N	f	f	t	f	t	\N	\N	2026-07-21 16:02:00.975786	2026-07-21 16:02:00.975786
858	858	1	bookarch/00004/Esse_i_razmyshleniya_o_Cheloveke_i_ego_Uchenii.zip	1026	bbae8a8368b17af0a2ef660005265a144e0e81a4229c01751ccccc2551155b81	\N	\N	f	f	t	f	t	\N	\N	2026-07-21 16:02:00.981325	2026-07-21 16:02:00.981325
859	859	1	bookarch/00004/Dao_de_tszin.zip	1362288	1fb23137c1ce5dcdc9568d5d68a8cca20da64d0546d0ca714781431011260380	\N	\N	f	f	t	f	t	\N	\N	2026-07-21 16:02:01.050855	2026-07-21 16:02:01.050855
860	860	3	bookarch/00004/Put_dzhedaya.zip	6568293	3367302fd6e00fedc14752295a52b1e719a71fae34110e7ea04ccae5f518d29c	\N	\N	f	f	t	f	t	\N	\N	2026-07-21 16:02:01.162583	2026-07-21 16:02:01.162583
861	861	6	bookarch/00004/Nazvanie_i_avtor_ukazannogo_teksta_ne_opredeleny__tak_kak_predstavleny_dopolnitelnye_knigi.zip	9184058	c8540f23e90a0c76055a79ed17eae9b2279d8223e806b6174e3531037f58751a	\N	\N	f	f	t	f	t	\N	\N	2026-07-21 16:02:13.116777	2026-07-21 16:02:13.116777
862	862	1	bookarch/00004/Uchenie_dona_Khuana__put_znaniya_indeytsev_yaki.zip	203908	6ee620485c40cb09a4532403a8d30b48943a643d193825a168933a1114a50b67	\N	\N	f	f	t	f	t	\N	\N	2026-07-21 16:02:13.150713	2026-07-21 16:02:13.150713
863	863	8	bookarch/00004/OTDELENNAYa_REALNOST__Kniga_2.zip	272322	c5671e760d2fe0fb2369ac5494542ae5ede62cf69e40ecfb74af95277d94fb71	\N	\N	f	f	t	f	t	\N	\N	2026-07-21 16:02:51.827047	2026-07-21 16:02:51.827047
864	864	8	bookarch/00004/Puteshestvie_v_Ikstlan__Put_k_znaniyu_i_sile_yogi_shamanizma_meksikanskikh_indeytsev.zip	287395	8d6fdf1a9eddc04bafac1e00f8101c8630bf8bfa653ae3d784e4a5f12d42a769	\N	\N	f	f	t	f	t	\N	\N	2026-07-21 16:03:37.246577	2026-07-21 16:03:37.246577
865	865	8	bookarch/00004/Skazki_o_sile__kniga_4.zip	294887	e11b496079a04ca1c34b1eab5d04db58c71e94c616b06103dc2058e7064d84b2	\N	\N	f	f	t	f	t	\N	\N	2026-07-21 16:04:13.680014	2026-07-21 16:04:13.680014
866	866	8	bookarch/00004/Vtoroe_koltso_sily__Perekreste_zhizney____Note__The_book_mentioned_is_part_5_of_a_larger_work__the_full_title_may_vary_in_different_editions.zip	314310	bb0ffe1595bfaab52c002ac85c6e993f6a5e7006f9b5bc4fc81a97cb503c0eef	\N	\N	f	f	t	f	t	\N	\N	2026-07-21 16:05:01.023192	2026-07-21 16:05:01.023192
867	867	8	bookarch/00004/Kniga_6__Dar_Orla.zip	324526	376fad3fc405bce1d4329c37398e42123050f220b43202de373cf93189dbb02c	\N	\N	f	f	t	f	t	\N	\N	2026-07-21 16:05:34.764247	2026-07-21 16:05:34.764247
868	868	8	bookarch/00004/Vnutrenniy_ogon__Kniga_sem__primenenie_levogo_yadra_v_povsednevnoy_zhizni.zip	281208	c97dd6d2957b05166ced7fc4c59e460a18f4d4ed0eeadb538f09e2a05ccb93de	\N	\N	f	f	t	f	t	\N	\N	2026-07-21 16:06:12.236656	2026-07-21 16:06:12.236656
869	869	8	bookarch/00004/SilaBezmolviya__Prolog.zip	255216	4949e60b60338a4b876a4d64a4de4ef27a1085e49b3c7761fac6ed9c37074dc4	\N	\N	f	f	t	f	t	\N	\N	2026-07-21 16:06:45.91521	2026-07-21 16:06:45.91521
870	870	8	bookarch/00004/Aktivnaya_storona_beskonechnosti__Tom_10__Knigi_o_masterstve.zip	282725	0b50ee64bb73b6f925135dbd970e5451719b692aceb2ea0433ce84b0acc4dbc3	\N	\N	f	f	t	f	t	\N	\N	2026-07-21 16:07:37.630942	2026-07-21 16:07:37.630942
872	872	1	bookarch/00004/Viktor_Pelevin_i_effekt_Pustoty.zip	13897	cc1cbbd6a7e433691f4adc0412b10633f7a3634dee7e3af552018374c9b508d1	\N	\N	f	f	t	f	t	\N	\N	2026-07-21 16:07:37.655156	2026-07-21 16:07:37.655156
873	873	1	bookarch/00004/Empire_V.zip	405718	d36fef1495eb0258a7543063f430d6cb1e1bf6e7e217f686db05a6a117e74abd	\N	\N	f	f	t	f	t	\N	\N	2026-07-21 16:07:37.681712	2026-07-21 16:07:37.681712
874	874	1	bookarch/00004/Generation__P.zip	274284	be0741f6e43211f15a7be6bfef97a2260bb1abb111a80aa3fd22b00fa85ada05	\N	\N	f	f	t	f	t	\N	\N	2026-07-21 16:07:37.72132	2026-07-21 16:07:37.72132
875	875	1	bookarch/00004/Relics__Rannee_i_neizdannoe__Sbornik.zip	263607	1a61d6afd01ccd7f1a3c8a9cb5d040e17e8a97054a2746a67636db78f63b8fc8	\N	\N	f	f	t	f	t	\N	\N	2026-07-21 16:07:37.758131	2026-07-21 16:07:37.758131
876	876	1	bookarch/00004/S_N_U_F_F.zip	476408	69a76907c7cc077f6827eb2b89687d13bfbf729eedb1592adfb8af317ba9c271	\N	\N	f	f	t	f	t	\N	\N	2026-07-21 16:07:37.817524	2026-07-21 16:07:37.817524
877	877	1	bookarch/00004/Timeout__ili_Vechernyaya_Moskva.zip	23367	21b683df4ca675deb45c5abd450179d99a9269d25ad1aaa72c14cba87612ff68	\N	\N	f	f	t	f	t	\N	\N	2026-07-21 16:07:37.826585	2026-07-21 16:07:37.826585
878	878	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-21 16:09:00.978004	2026-07-21 16:09:00.978004
879	879	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-21 16:09:00.984465	2026-07-21 16:09:00.984465
880	880	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-21 16:09:00.997458	2026-07-21 16:09:00.997458
882	882	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-21 16:09:01.036043	2026-07-21 16:09:01.036043
883	883	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-21 16:09:01.060855	2026-07-21 16:09:01.060855
884	884	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-21 16:09:01.078846	2026-07-21 16:09:01.078846
885	885	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-21 16:09:01.094962	2026-07-21 16:09:01.094962
886	886	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-21 16:09:01.101245	2026-07-21 16:09:01.101245
887	887	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-21 16:09:01.117888	2026-07-21 16:09:01.117888
888	888	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-21 16:09:01.141347	2026-07-21 16:09:01.141347
889	889	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-21 16:09:01.15954	2026-07-21 16:09:01.15954
890	890	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-21 16:09:01.657874	2026-07-21 16:09:01.657874
891	891	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-21 16:09:01.677387	2026-07-21 16:09:01.677387
892	892	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-21 16:09:01.69528	2026-07-21 16:09:01.69528
893	893	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-21 16:09:01.71826	2026-07-21 16:09:01.71826
894	894	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-21 16:09:01.734127	2026-07-21 16:09:01.734127
895	895	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-21 20:55:27.680959	2026-07-21 20:55:27.680959
896	896	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-21 20:55:27.687234	2026-07-21 20:55:27.687234
897	897	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-21 20:55:27.698365	2026-07-21 20:55:27.698365
899	899	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-21 20:55:27.733729	2026-07-21 20:55:27.733729
900	900	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-21 20:55:27.756819	2026-07-21 20:55:27.756819
901	901	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-21 20:55:27.775829	2026-07-21 20:55:27.775829
902	902	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-21 20:55:27.792904	2026-07-21 20:55:27.792904
903	903	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-21 20:55:27.799325	2026-07-21 20:55:27.799325
904	904	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-21 20:55:27.811507	2026-07-21 20:55:27.811507
905	905	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-21 20:55:27.828743	2026-07-21 20:55:27.828743
906	906	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-21 20:55:27.846384	2026-07-21 20:55:27.846384
907	907	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-21 20:55:28.306723	2026-07-21 20:55:28.306723
908	908	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-21 20:55:28.325034	2026-07-21 20:55:28.325034
909	909	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-21 20:55:28.342165	2026-07-21 20:55:28.342165
910	910	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-21 20:55:28.358562	2026-07-21 20:55:28.358562
911	911	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-21 20:55:28.374795	2026-07-21 20:55:28.374795
912	912	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-21 21:23:05.812351	2026-07-21 21:23:05.812351
913	913	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-21 21:23:05.818941	2026-07-21 21:23:05.818941
914	914	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-21 21:23:05.83424	2026-07-21 21:23:05.83424
916	916	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-21 21:23:05.872347	2026-07-21 21:23:05.872347
917	917	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-21 21:23:05.89653	2026-07-21 21:23:05.89653
918	918	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-21 21:23:05.91756	2026-07-21 21:23:05.91756
919	919	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-21 21:23:05.93518	2026-07-21 21:23:05.93518
920	920	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-21 21:23:05.941308	2026-07-21 21:23:05.941308
921	921	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-21 21:23:05.955498	2026-07-21 21:23:05.955498
922	922	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-21 21:23:05.974623	2026-07-21 21:23:05.974623
923	923	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-21 21:23:05.991554	2026-07-21 21:23:05.991554
924	924	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-21 21:23:06.482959	2026-07-21 21:23:06.482959
925	925	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-21 21:23:06.501982	2026-07-21 21:23:06.501982
926	926	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-21 21:23:06.520624	2026-07-21 21:23:06.520624
927	927	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-21 21:23:06.538302	2026-07-21 21:23:06.538302
928	928	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-21 21:23:06.552965	2026-07-21 21:23:06.552965
929	929	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-22 09:33:47.713172	2026-07-22 09:33:47.713172
930	930	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-22 09:33:47.719627	2026-07-22 09:33:47.719627
931	931	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-22 09:33:47.73352	2026-07-22 09:33:47.73352
933	933	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-22 09:33:47.774321	2026-07-22 09:33:47.774321
934	934	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-22 09:33:47.79992	2026-07-22 09:33:47.79992
935	935	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-22 09:33:47.82124	2026-07-22 09:33:47.82124
936	936	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-22 09:33:47.840991	2026-07-22 09:33:47.840991
937	937	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-22 09:33:47.847151	2026-07-22 09:33:47.847151
938	938	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-22 09:33:47.863741	2026-07-22 09:33:47.863741
939	939	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-22 09:33:47.888197	2026-07-22 09:33:47.888197
940	940	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-22 09:33:52.900853	2026-07-22 09:33:52.900853
941	941	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-22 09:33:53.384278	2026-07-22 09:33:53.384278
942	942	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-22 09:33:53.402597	2026-07-22 09:33:53.402597
943	943	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-22 09:33:53.426572	2026-07-22 09:33:53.426572
944	944	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-22 09:33:53.443721	2026-07-22 09:33:53.443721
945	945	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-22 09:33:53.460587	2026-07-22 09:33:53.460587
946	946	1	bookarch/00004/Detskiy_mir__sbornik.zip	580994	9e955443598da04d26a221b954f81605207c63df5d5551c8a5047357ef1526e8	\N	\N	f	f	t	f	t	\N	\N	2026-07-23 07:07:55.587012	2026-07-23 07:07:55.587012
947	947	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-23 07:17:39.553704	2026-07-23 07:17:39.553704
948	948	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-23 07:17:39.561426	2026-07-23 07:17:39.561426
949	949	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-23 07:17:39.577495	2026-07-23 07:17:39.577495
954	954	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-23 07:17:39.675216	2026-07-23 07:17:39.675216
955	955	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-23 07:17:39.681092	2026-07-23 07:17:39.681092
956	956	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-23 07:17:39.694974	2026-07-23 07:17:39.694974
957	957	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-23 07:17:39.716267	2026-07-23 07:17:39.716267
958	958	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-23 07:17:39.733793	2026-07-23 07:17:39.733793
959	959	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-23 07:17:40.237189	2026-07-23 07:17:40.237189
960	960	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-23 07:17:40.254592	2026-07-23 07:17:40.254592
961	961	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-23 07:17:40.272656	2026-07-23 07:17:40.272656
962	962	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-23 07:17:40.289769	2026-07-23 07:17:40.289769
963	963	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-23 07:17:40.306254	2026-07-23 07:17:40.306254
964	964	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-24 13:13:24.566758	2026-07-24 13:13:24.566758
965	965	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-24 13:13:24.573495	2026-07-24 13:13:24.573495
966	966	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-24 13:13:24.58461	2026-07-24 13:13:24.58461
968	968	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-24 13:13:24.622676	2026-07-24 13:13:24.622676
969	969	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-24 13:13:24.646203	2026-07-24 13:13:24.646203
970	970	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-24 13:13:24.872152	2026-07-24 13:13:24.872152
971	971	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-24 13:13:24.888625	2026-07-24 13:13:24.888625
972	972	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-24 13:13:24.895048	2026-07-24 13:13:24.895048
973	973	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-24 13:13:24.908016	2026-07-24 13:13:24.908016
974	974	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-24 13:13:24.926932	2026-07-24 13:13:24.926932
975	975	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-24 13:13:24.943588	2026-07-24 13:13:24.943588
976	976	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-24 13:13:25.455177	2026-07-24 13:13:25.455177
977	977	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-24 13:13:25.473525	2026-07-24 13:13:25.473525
978	978	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-24 13:13:25.490529	2026-07-24 13:13:25.490529
979	979	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-24 13:13:25.509177	2026-07-24 13:13:25.509177
980	980	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-24 13:13:25.526854	2026-07-24 13:13:25.526854
981	981	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-24 13:18:50.692058	2026-07-24 13:18:50.692058
982	982	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-24 13:18:50.698837	2026-07-24 13:18:50.698837
983	983	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-24 13:18:50.711429	2026-07-24 13:18:50.711429
985	985	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-24 13:18:50.751683	2026-07-24 13:18:50.751683
986	986	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-24 13:18:50.777676	2026-07-24 13:18:50.777676
987	987	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-24 13:18:50.798002	2026-07-24 13:18:50.798002
988	988	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-24 13:18:50.815429	2026-07-24 13:18:50.815429
989	989	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-24 13:18:50.822616	2026-07-24 13:18:50.822616
990	990	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-24 13:18:50.839801	2026-07-24 13:18:50.839801
991	991	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-24 13:18:50.859227	2026-07-24 13:18:50.859227
992	992	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-24 13:18:50.879886	2026-07-24 13:18:50.879886
993	993	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-24 13:18:51.419246	2026-07-24 13:18:51.419246
994	994	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-24 13:18:51.441838	2026-07-24 13:18:51.441838
995	995	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-24 13:18:51.460151	2026-07-24 13:18:51.460151
996	996	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-24 13:18:51.476998	2026-07-24 13:18:51.476998
997	997	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-24 13:18:51.496996	2026-07-24 13:18:51.496996
998	998	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-24 13:36:44.535594	2026-07-24 13:36:44.535594
999	999	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-24 13:36:44.541781	2026-07-24 13:36:44.541781
1000	1000	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-24 13:36:44.552856	2026-07-24 13:36:44.552856
1002	1002	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-24 13:36:44.586251	2026-07-24 13:36:44.586251
1003	1003	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-24 13:36:44.610108	2026-07-24 13:36:44.610108
1004	1004	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-24 13:36:44.628097	2026-07-24 13:36:44.628097
1005	1005	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-24 13:36:44.645743	2026-07-24 13:36:44.645743
1006	1006	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-24 13:36:44.651616	2026-07-24 13:36:44.651616
1007	1007	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-24 13:36:44.667209	2026-07-24 13:36:44.667209
1008	1008	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-24 13:36:44.685492	2026-07-24 13:36:44.685492
1009	1009	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-24 13:36:44.702462	2026-07-24 13:36:44.702462
1010	1010	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-24 13:36:45.194849	2026-07-24 13:36:45.194849
1011	1011	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-24 13:36:45.212671	2026-07-24 13:36:45.212671
1012	1012	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-24 13:36:45.228532	2026-07-24 13:36:45.228532
1013	1013	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-24 13:36:45.24484	2026-07-24 13:36:45.24484
1014	1014	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-24 13:36:45.26059	2026-07-24 13:36:45.26059
1015	1015	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-24 15:09:43.910989	2026-07-24 15:09:43.910989
1016	1016	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-24 15:09:43.917654	2026-07-24 15:09:43.917654
1017	1017	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-24 15:09:43.931523	2026-07-24 15:09:43.931523
1019	1019	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-24 15:09:43.965619	2026-07-24 15:09:43.965619
1020	1020	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-24 15:09:43.991359	2026-07-24 15:09:43.991359
1021	1021	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-24 15:09:44.012588	2026-07-24 15:09:44.012588
1022	1022	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-24 15:09:44.032559	2026-07-24 15:09:44.032559
1023	1023	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-24 15:09:44.038459	2026-07-24 15:09:44.038459
1024	1024	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-24 15:09:44.053328	2026-07-24 15:09:44.053328
1025	1025	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-24 15:09:44.072821	2026-07-24 15:09:44.072821
1026	1026	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-24 15:09:44.089424	2026-07-24 15:09:44.089424
1027	1027	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-24 15:09:44.619403	2026-07-24 15:09:44.619403
1028	1028	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-24 15:09:44.638413	2026-07-24 15:09:44.638413
1029	1029	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-24 15:09:44.655964	2026-07-24 15:09:44.655964
1030	1030	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-24 15:09:44.672235	2026-07-24 15:09:44.672235
1031	1031	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-24 15:09:44.68828	2026-07-24 15:09:44.68828
1032	1032	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-24 15:41:19.686104	2026-07-24 15:41:19.686104
1033	1033	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-24 15:41:19.692653	2026-07-24 15:41:19.692653
1034	1034	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-24 15:41:19.707093	2026-07-24 15:41:19.707093
1036	1036	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-24 15:41:19.742577	2026-07-24 15:41:19.742577
1037	1037	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-24 15:41:19.767177	2026-07-24 15:41:19.767177
1038	1038	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-24 15:41:19.78451	2026-07-24 15:41:19.78451
1039	1039	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-24 15:41:19.800165	2026-07-24 15:41:19.800165
1040	1040	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-24 15:41:19.806294	2026-07-24 15:41:19.806294
1041	1041	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-24 15:41:19.818826	2026-07-24 15:41:19.818826
1042	1042	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-24 15:41:19.836339	2026-07-24 15:41:19.836339
1043	1043	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-24 15:41:19.853324	2026-07-24 15:41:19.853324
1044	1044	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-24 15:41:20.337568	2026-07-24 15:41:20.337568
1045	1045	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-24 15:41:20.354362	2026-07-24 15:41:20.354362
1046	1046	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-24 15:41:20.371216	2026-07-24 15:41:20.371216
1047	1047	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-24 15:41:20.387168	2026-07-24 15:41:20.387168
1048	1048	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-24 15:41:20.404612	2026-07-24 15:41:20.404612
1049	1049	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-24 15:46:46.824363	2026-07-24 15:46:46.824363
1050	1050	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-24 15:46:46.830658	2026-07-24 15:46:46.830658
1051	1051	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-24 15:46:46.842188	2026-07-24 15:46:46.842188
1053	1053	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-24 15:46:46.878097	2026-07-24 15:46:46.878097
1054	1054	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-24 15:46:46.899645	2026-07-24 15:46:46.899645
1055	1055	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-24 15:46:46.91668	2026-07-24 15:46:46.91668
1056	1056	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-24 15:46:46.931845	2026-07-24 15:46:46.931845
1057	1057	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-24 15:46:46.93769	2026-07-24 15:46:46.93769
1058	1058	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-24 15:46:46.951962	2026-07-24 15:46:46.951962
1059	1059	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-24 15:46:46.969325	2026-07-24 15:46:46.969325
1060	1060	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-24 15:46:46.986683	2026-07-24 15:46:46.986683
1061	1061	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-24 15:46:47.452354	2026-07-24 15:46:47.452354
1062	1062	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-24 15:46:47.470691	2026-07-24 15:46:47.470691
1063	1063	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-24 15:46:47.490046	2026-07-24 15:46:47.490046
1064	1064	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-24 15:46:47.509323	2026-07-24 15:46:47.509323
1065	1065	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-24 15:46:47.527781	2026-07-24 15:46:47.527781
1066	1066	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-24 15:53:08.595811	2026-07-24 15:53:08.595811
1067	1067	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-24 15:53:08.602755	2026-07-24 15:53:08.602755
1068	1068	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-24 15:53:08.615847	2026-07-24 15:53:08.615847
1070	1070	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-24 15:53:08.650741	2026-07-24 15:53:08.650741
1071	1071	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-24 15:53:08.673799	2026-07-24 15:53:08.673799
1072	1072	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-24 15:53:08.692076	2026-07-24 15:53:08.692076
1073	1073	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-24 15:53:08.709321	2026-07-24 15:53:08.709321
1074	1074	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-24 15:53:08.715887	2026-07-24 15:53:08.715887
1075	1075	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-24 15:53:08.727616	2026-07-24 15:53:08.727616
1076	1076	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-24 15:53:08.746351	2026-07-24 15:53:08.746351
1077	1077	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-24 15:53:08.762448	2026-07-24 15:53:08.762448
1078	1078	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-24 15:53:09.246651	2026-07-24 15:53:09.246651
1079	1079	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-24 15:53:09.264586	2026-07-24 15:53:09.264586
1080	1080	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-24 15:53:09.281413	2026-07-24 15:53:09.281413
1081	1081	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-24 15:53:09.298026	2026-07-24 15:53:09.298026
1082	1082	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-24 15:53:09.31492	2026-07-24 15:53:09.31492
1083	1083	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-24 15:54:21.473135	2026-07-24 15:54:21.473135
1084	1084	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-24 15:54:21.479475	2026-07-24 15:54:21.479475
1085	1085	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-24 15:54:21.492925	2026-07-24 15:54:21.492925
1087	1087	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-24 15:54:21.532338	2026-07-24 15:54:21.532338
1088	1088	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-24 15:54:21.555972	2026-07-24 15:54:21.555972
1089	1089	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-24 15:54:21.573318	2026-07-24 15:54:21.573318
1090	1090	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-24 15:54:21.591606	2026-07-24 15:54:21.591606
1091	1091	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-24 15:54:21.597821	2026-07-24 15:54:21.597821
1092	1092	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-24 15:54:21.61436	2026-07-24 15:54:21.61436
1093	1093	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-24 15:54:21.636086	2026-07-24 15:54:21.636086
1094	1094	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-24 15:54:21.656271	2026-07-24 15:54:21.656271
1095	1095	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-24 15:54:22.157474	2026-07-24 15:54:22.157474
1096	1096	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-24 15:54:22.177833	2026-07-24 15:54:22.177833
1097	1097	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-24 15:54:22.195172	2026-07-24 15:54:22.195172
1098	1098	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-24 15:54:22.212479	2026-07-24 15:54:22.212479
1099	1099	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-24 15:54:22.228632	2026-07-24 15:54:22.228632
1100	1100	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-24 19:06:25.640595	2026-07-24 19:06:25.640595
1101	1101	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-24 19:06:25.647304	2026-07-24 19:06:25.647304
1102	1102	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-24 19:06:25.660861	2026-07-24 19:06:25.660861
1104	1104	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-24 19:06:25.700338	2026-07-24 19:06:25.700338
1105	1105	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-24 19:06:25.723716	2026-07-24 19:06:25.723716
1106	1106	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-24 19:06:25.746073	2026-07-24 19:06:25.746073
1107	1107	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-24 19:06:25.767342	2026-07-24 19:06:25.767342
1108	1108	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-24 19:06:25.773304	2026-07-24 19:06:25.773304
1109	1109	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-24 19:06:25.788612	2026-07-24 19:06:25.788612
1110	1110	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-24 19:06:25.808211	2026-07-24 19:06:25.808211
1111	1111	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-24 19:06:25.827394	2026-07-24 19:06:25.827394
1112	1112	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-24 19:06:26.375313	2026-07-24 19:06:26.375313
1113	1113	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-24 19:06:26.393159	2026-07-24 19:06:26.393159
1114	1114	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-24 19:06:26.413001	2026-07-24 19:06:26.413001
1115	1115	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-24 19:06:26.430468	2026-07-24 19:06:26.430468
1116	1116	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-24 19:06:26.446678	2026-07-24 19:06:26.446678
1117	1117	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-24 19:13:00.870128	2026-07-24 19:13:00.870128
1118	1118	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-24 19:13:00.877423	2026-07-24 19:13:00.877423
1119	1119	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-24 19:13:00.89039	2026-07-24 19:13:00.89039
1121	1121	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-24 19:13:00.927296	2026-07-24 19:13:00.927296
1122	1122	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-24 19:13:00.950338	2026-07-24 19:13:00.950338
1123	1123	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-24 19:13:00.974985	2026-07-24 19:13:00.974985
1124	1124	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-24 19:13:00.990697	2026-07-24 19:13:00.990697
1125	1125	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-24 19:13:00.996987	2026-07-24 19:13:00.996987
1126	1126	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-24 19:13:01.009138	2026-07-24 19:13:01.009138
1127	1127	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-24 19:13:01.026899	2026-07-24 19:13:01.026899
1128	1128	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-24 19:13:01.045419	2026-07-24 19:13:01.045419
1129	1129	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-24 19:13:01.568701	2026-07-24 19:13:01.568701
1130	1130	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-24 19:13:01.586883	2026-07-24 19:13:01.586883
1131	1131	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-24 19:13:01.609914	2026-07-24 19:13:01.609914
1132	1132	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-24 19:13:01.628404	2026-07-24 19:13:01.628404
1133	1133	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-24 19:13:01.649206	2026-07-24 19:13:01.649206
1134	1134	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-24 19:47:46.415981	2026-07-24 19:47:46.415981
1135	1135	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-24 19:47:46.422245	2026-07-24 19:47:46.422245
1136	1136	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-24 19:47:46.433338	2026-07-24 19:47:46.433338
1138	1138	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-24 19:47:46.471255	2026-07-24 19:47:46.471255
1139	1139	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-24 19:47:46.493131	2026-07-24 19:47:46.493131
1140	1140	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-24 19:47:46.510162	2026-07-24 19:47:46.510162
1141	1141	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-24 19:47:46.525577	2026-07-24 19:47:46.525577
1142	1142	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-24 19:47:46.531306	2026-07-24 19:47:46.531306
1143	1143	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-24 19:47:46.544987	2026-07-24 19:47:46.544987
1144	1144	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-24 19:47:46.566817	2026-07-24 19:47:46.566817
1145	1145	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-24 19:47:46.582561	2026-07-24 19:47:46.582561
1146	1146	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-24 19:47:47.086925	2026-07-24 19:47:47.086925
1147	1147	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-24 19:47:47.105484	2026-07-24 19:47:47.105484
1148	1148	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-24 19:47:47.131975	2026-07-24 19:47:47.131975
1149	1149	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-24 19:47:47.14891	2026-07-24 19:47:47.14891
1150	1150	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-24 19:47:47.163993	2026-07-24 19:47:47.163993
1151	1151	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-24 19:48:03.634369	2026-07-24 19:48:03.634369
1152	1152	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-24 19:48:03.641471	2026-07-24 19:48:03.641471
1153	1153	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-24 19:48:03.654486	2026-07-24 19:48:03.654486
1155	1155	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-24 19:48:03.691668	2026-07-24 19:48:03.691668
1156	1156	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-24 19:48:03.715392	2026-07-24 19:48:03.715392
1157	1157	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-24 19:48:03.735367	2026-07-24 19:48:03.735367
1158	1158	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-24 19:48:03.752975	2026-07-24 19:48:03.752975
1159	1159	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-24 19:48:03.759338	2026-07-24 19:48:03.759338
1160	1160	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-24 19:48:03.771414	2026-07-24 19:48:03.771414
1161	1161	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-24 19:48:03.790649	2026-07-24 19:48:03.790649
1162	1162	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-24 19:48:03.819755	2026-07-24 19:48:03.819755
1163	1163	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-24 19:48:04.339376	2026-07-24 19:48:04.339376
1164	1164	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-24 19:48:04.356463	2026-07-24 19:48:04.356463
1165	1165	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-24 19:48:04.379497	2026-07-24 19:48:04.379497
1166	1166	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-24 19:48:04.39956	2026-07-24 19:48:04.39956
1167	1167	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-24 19:48:04.418486	2026-07-24 19:48:04.418486
1168	1168	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-25 11:35:26.597435	2026-07-25 11:35:26.597435
1169	1169	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-25 11:35:26.618086	2026-07-25 11:35:26.618086
1170	1170	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-25 11:35:26.629088	2026-07-25 11:35:26.629088
1172	1172	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-25 11:35:26.670686	2026-07-25 11:35:26.670686
1173	1173	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-25 11:35:26.695544	2026-07-25 11:35:26.695544
1174	1174	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-25 11:35:26.71329	2026-07-25 11:35:26.71329
1175	1175	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-25 11:35:26.729228	2026-07-25 11:35:26.729228
1176	1176	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-25 11:35:26.737323	2026-07-25 11:35:26.737323
1177	1177	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-25 11:35:26.7503	2026-07-25 11:35:26.7503
1178	1178	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-25 11:35:26.767563	2026-07-25 11:35:26.767563
1179	1179	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-25 11:35:26.785185	2026-07-25 11:35:26.785185
1180	1180	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-25 11:35:27.277602	2026-07-25 11:35:27.277602
1181	1181	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-25 11:35:27.295264	2026-07-25 11:35:27.295264
1182	1182	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-25 11:35:27.312914	2026-07-25 11:35:27.312914
1183	1183	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-25 11:35:27.328963	2026-07-25 11:35:27.328963
1184	1184	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-25 11:35:27.345399	2026-07-25 11:35:27.345399
1185	1185	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-25 14:19:39.849752	2026-07-25 14:19:39.849752
1186	1186	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-25 14:19:39.858317	2026-07-25 14:19:39.858317
1187	1187	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-25 14:19:39.871557	2026-07-25 14:19:39.871557
1189	1189	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-25 14:19:39.911022	2026-07-25 14:19:39.911022
1190	1190	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-25 14:19:39.940497	2026-07-25 14:19:39.940497
1191	1191	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-25 14:19:39.957652	2026-07-25 14:19:39.957652
1192	1192	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-25 14:19:39.974833	2026-07-25 14:19:39.974833
1193	1193	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-25 14:19:39.980905	2026-07-25 14:19:39.980905
1194	1194	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-25 14:19:39.994795	2026-07-25 14:19:39.994795
1195	1195	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-25 14:19:40.014643	2026-07-25 14:19:40.014643
1196	1196	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-25 14:19:40.03149	2026-07-25 14:19:40.03149
1197	1197	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-25 14:19:40.517551	2026-07-25 14:19:40.517551
1198	1198	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-25 14:19:40.536549	2026-07-25 14:19:40.536549
1199	1199	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-25 14:19:40.556586	2026-07-25 14:19:40.556586
1200	1200	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-25 14:19:40.574113	2026-07-25 14:19:40.574113
1201	1201	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-25 14:19:40.59136	2026-07-25 14:19:40.59136
1202	1202	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-25 15:04:15.330683	2026-07-25 15:04:15.330683
1203	1203	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-25 15:04:15.33773	2026-07-25 15:04:15.33773
1204	1204	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-25 15:04:15.349899	2026-07-25 15:04:15.349899
1206	1206	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-25 15:04:15.388347	2026-07-25 15:04:15.388347
1207	1207	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-25 15:04:15.410197	2026-07-25 15:04:15.410197
1208	1208	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-25 15:04:15.429545	2026-07-25 15:04:15.429545
1209	1209	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-25 15:04:15.449545	2026-07-25 15:04:15.449545
1210	1210	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-25 15:04:15.455178	2026-07-25 15:04:15.455178
1211	1211	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-25 15:04:15.467931	2026-07-25 15:04:15.467931
1212	1212	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-25 15:04:15.485409	2026-07-25 15:04:15.485409
1213	1213	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-25 15:04:15.501876	2026-07-25 15:04:15.501876
1214	1214	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-25 15:04:16.008767	2026-07-25 15:04:16.008767
1215	1215	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-25 15:04:16.027255	2026-07-25 15:04:16.027255
1216	1216	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-25 15:04:16.044203	2026-07-25 15:04:16.044203
1217	1217	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-25 15:04:16.060537	2026-07-25 15:04:16.060537
1218	1218	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-25 15:04:16.078449	2026-07-25 15:04:16.078449
1219	1219	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-25 15:29:01.853788	2026-07-25 15:29:01.853788
1220	1220	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-25 15:29:01.86069	2026-07-25 15:29:01.86069
1221	1221	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-25 15:29:01.872967	2026-07-25 15:29:01.872967
1223	1223	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-25 15:29:01.909197	2026-07-25 15:29:01.909197
1224	1224	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-25 15:29:01.932952	2026-07-25 15:29:01.932952
1225	1225	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-25 15:29:01.949971	2026-07-25 15:29:01.949971
1226	1226	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-25 15:29:01.975503	2026-07-25 15:29:01.975503
1227	1227	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-25 15:29:01.982144	2026-07-25 15:29:01.982144
1228	1228	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-25 15:29:01.994111	2026-07-25 15:29:01.994111
1229	1229	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-25 15:29:02.012812	2026-07-25 15:29:02.012812
1230	1230	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-25 15:29:02.029	2026-07-25 15:29:02.029
1231	1231	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-25 15:29:02.519947	2026-07-25 15:29:02.519947
1232	1232	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-25 15:29:02.538807	2026-07-25 15:29:02.538807
1233	1233	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-25 15:29:02.557888	2026-07-25 15:29:02.557888
1234	1234	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-25 15:29:02.574292	2026-07-25 15:29:02.574292
1235	1235	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-25 15:29:02.589087	2026-07-25 15:29:02.589087
1236	1236	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-25 15:47:11.629139	2026-07-25 15:47:11.629139
1237	1237	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-25 15:47:11.635446	2026-07-25 15:47:11.635446
1238	1238	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-25 15:47:11.65204	2026-07-25 15:47:11.65204
1240	1240	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-25 15:47:11.692465	2026-07-25 15:47:11.692465
1241	1241	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-25 15:47:11.716246	2026-07-25 15:47:11.716246
1242	1242	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-25 15:47:11.735915	2026-07-25 15:47:11.735915
1243	1243	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-25 15:47:11.754176	2026-07-25 15:47:11.754176
1244	1244	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-25 15:47:11.760246	2026-07-25 15:47:11.760246
1245	1245	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-25 15:47:11.781467	2026-07-25 15:47:11.781467
1246	1246	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-25 15:47:11.801097	2026-07-25 15:47:11.801097
1247	1247	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-25 15:47:11.820388	2026-07-25 15:47:11.820388
1248	1248	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-25 15:47:12.333721	2026-07-25 15:47:12.333721
1249	1249	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-25 15:47:12.360108	2026-07-25 15:47:12.360108
1250	1250	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-25 15:47:12.393971	2026-07-25 15:47:12.393971
1251	1251	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-25 15:47:12.414296	2026-07-25 15:47:12.414296
1252	1252	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-25 15:47:12.442687	2026-07-25 15:47:12.442687
1253	1253	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-25 15:54:34.743911	2026-07-25 15:54:34.743911
1254	1254	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-25 15:54:34.749968	2026-07-25 15:54:34.749968
1255	1255	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-25 15:54:34.760406	2026-07-25 15:54:34.760406
1257	1257	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-25 15:54:34.793323	2026-07-25 15:54:34.793323
1258	1258	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-25 15:54:34.816539	2026-07-25 15:54:34.816539
1259	1259	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-25 15:54:34.834187	2026-07-25 15:54:34.834187
1260	1260	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-25 15:54:34.848658	2026-07-25 15:54:34.848658
1261	1261	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-25 15:54:34.854424	2026-07-25 15:54:34.854424
1262	1262	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-25 15:54:34.867163	2026-07-25 15:54:34.867163
1263	1263	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-25 15:54:34.887096	2026-07-25 15:54:34.887096
1264	1264	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-25 15:54:34.904952	2026-07-25 15:54:34.904952
1265	1265	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-25 15:54:35.392974	2026-07-25 15:54:35.392974
1266	1266	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-25 15:54:35.411116	2026-07-25 15:54:35.411116
1267	1267	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-25 15:54:35.42709	2026-07-25 15:54:35.42709
1268	1268	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-25 15:54:35.444918	2026-07-25 15:54:35.444918
1269	1269	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-25 15:54:35.463791	2026-07-25 15:54:35.463791
1270	1270	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-26 19:34:10.629768	2026-07-26 19:34:10.629768
1271	1271	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-26 19:34:10.64703	2026-07-26 19:34:10.64703
1272	1272	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-26 19:34:10.665086	2026-07-26 19:34:10.665086
1274	1274	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-26 19:34:10.711772	2026-07-26 19:34:10.711772
1275	1275	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-26 19:34:10.738556	2026-07-26 19:34:10.738556
1276	1276	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-26 19:34:10.760565	2026-07-26 19:34:10.760565
1277	1277	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-26 19:34:10.778269	2026-07-26 19:34:10.778269
1278	1278	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-26 19:34:10.784682	2026-07-26 19:34:10.784682
1279	1279	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-26 19:34:10.799533	2026-07-26 19:34:10.799533
1280	1280	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-26 19:34:10.818841	2026-07-26 19:34:10.818841
1281	1281	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-26 19:34:10.84004	2026-07-26 19:34:10.84004
1282	1282	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-26 19:34:11.40391	2026-07-26 19:34:11.40391
1283	1283	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-26 19:34:11.42334	2026-07-26 19:34:11.42334
1284	1284	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-26 19:34:11.44492	2026-07-26 19:34:11.44492
1285	1285	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-26 19:34:11.46355	2026-07-26 19:34:11.46355
1286	1286	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-26 19:34:11.480723	2026-07-26 19:34:11.480723
1287	1287	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-26 20:03:20.490721	2026-07-26 20:03:20.490721
1288	1288	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-26 20:03:20.497074	2026-07-26 20:03:20.497074
1289	1289	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-26 20:03:20.508698	2026-07-26 20:03:20.508698
1291	1291	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-26 20:03:20.545579	2026-07-26 20:03:20.545579
1292	1292	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-26 20:03:20.571758	2026-07-26 20:03:20.571758
1293	1293	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-26 20:03:20.588725	2026-07-26 20:03:20.588725
1294	1294	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-26 20:03:20.604812	2026-07-26 20:03:20.604812
1295	1295	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-26 20:03:20.611036	2026-07-26 20:03:20.611036
1296	1296	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-26 20:03:20.623674	2026-07-26 20:03:20.623674
1297	1297	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-26 20:03:20.640799	2026-07-26 20:03:20.640799
1298	1298	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-26 20:03:20.656595	2026-07-26 20:03:20.656595
1299	1299	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-26 20:03:21.218299	2026-07-26 20:03:21.218299
1300	1300	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-26 20:03:21.235127	2026-07-26 20:03:21.235127
1301	1301	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-26 20:03:21.255066	2026-07-26 20:03:21.255066
1302	1302	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-26 20:03:21.271093	2026-07-26 20:03:21.271093
1303	1303	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-26 20:03:21.286791	2026-07-26 20:03:21.286791
1304	1304	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-26 20:50:17.488358	2026-07-26 20:50:17.488358
1305	1305	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-26 20:50:17.497292	2026-07-26 20:50:17.497292
1306	1306	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-26 20:50:17.50793	2026-07-26 20:50:17.50793
1308	1308	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-26 20:50:17.546186	2026-07-26 20:50:17.546186
1309	1309	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-26 20:50:17.573667	2026-07-26 20:50:17.573667
1310	1310	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-26 20:50:17.594869	2026-07-26 20:50:17.594869
1311	1311	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-26 20:50:17.611735	2026-07-26 20:50:17.611735
1312	1312	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-26 20:50:17.617712	2026-07-26 20:50:17.617712
1313	1313	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-26 20:50:17.630028	2026-07-26 20:50:17.630028
1314	1314	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-26 20:50:17.647073	2026-07-26 20:50:17.647073
1315	1315	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-26 20:50:17.666102	2026-07-26 20:50:17.666102
1316	1316	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-26 20:50:18.183151	2026-07-26 20:50:18.183151
1317	1317	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-26 20:50:18.204483	2026-07-26 20:50:18.204483
1318	1318	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-26 20:50:18.223243	2026-07-26 20:50:18.223243
1319	1319	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-26 20:50:18.239006	2026-07-26 20:50:18.239006
1320	1320	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-26 20:50:18.25431	2026-07-26 20:50:18.25431
1321	1321	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-26 21:03:21.375001	2026-07-26 21:03:21.375001
1322	1322	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-26 21:03:21.381478	2026-07-26 21:03:21.381478
1323	1323	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-26 21:03:21.393274	2026-07-26 21:03:21.393274
1325	1325	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-26 21:03:21.427703	2026-07-26 21:03:21.427703
1326	1326	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-26 21:03:21.451382	2026-07-26 21:03:21.451382
1327	1327	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-26 21:03:21.469133	2026-07-26 21:03:21.469133
1328	1328	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-26 21:03:21.486114	2026-07-26 21:03:21.486114
1329	1329	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-26 21:03:21.491856	2026-07-26 21:03:21.491856
1330	1330	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-26 21:03:21.504041	2026-07-26 21:03:21.504041
1331	1331	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-26 21:03:21.52175	2026-07-26 21:03:21.52175
1332	1332	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-26 21:03:21.538295	2026-07-26 21:03:21.538295
1333	1333	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-26 21:03:22.038481	2026-07-26 21:03:22.038481
1334	1334	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-26 21:03:22.055907	2026-07-26 21:03:22.055907
1335	1335	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-26 21:03:22.072202	2026-07-26 21:03:22.072202
1336	1336	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-26 21:03:22.091257	2026-07-26 21:03:22.091257
1337	1337	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-26 21:03:22.107418	2026-07-26 21:03:22.107418
1338	1338	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-27 07:08:39.906875	2026-07-27 07:08:39.906875
1339	1339	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-27 07:08:39.913707	2026-07-27 07:08:39.913707
1340	1340	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-27 07:08:39.924879	2026-07-27 07:08:39.924879
1342	1342	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-27 07:08:39.963112	2026-07-27 07:08:39.963112
1343	1343	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-27 07:08:39.98599	2026-07-27 07:08:39.98599
1344	1344	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-27 07:08:40.004416	2026-07-27 07:08:40.004416
1345	1345	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-27 07:08:40.020814	2026-07-27 07:08:40.020814
1346	1346	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-27 07:08:40.027235	2026-07-27 07:08:40.027235
1347	1347	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-27 07:08:40.041454	2026-07-27 07:08:40.041454
1348	1348	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-27 07:08:40.061855	2026-07-27 07:08:40.061855
1349	1349	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-27 07:08:40.079275	2026-07-27 07:08:40.079275
1350	1350	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-27 07:08:41.210459	2026-07-27 07:08:41.210459
1351	1351	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-27 07:08:41.226276	2026-07-27 07:08:41.226276
1352	1352	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-27 07:08:41.242867	2026-07-27 07:08:41.242867
1353	1353	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-27 07:08:41.25961	2026-07-27 07:08:41.25961
1354	1354	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-27 07:08:41.277766	2026-07-27 07:08:41.277766
1355	1355	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-27 07:14:51.216994	2026-07-27 07:14:51.216994
1356	1356	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-27 07:14:51.224802	2026-07-27 07:14:51.224802
1357	1357	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-27 07:14:51.240208	2026-07-27 07:14:51.240208
1359	1359	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-27 07:14:51.2788	2026-07-27 07:14:51.2788
1360	1360	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-27 07:14:51.302791	2026-07-27 07:14:51.302791
1361	1361	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-27 07:14:51.32056	2026-07-27 07:14:51.32056
1362	1362	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-27 07:14:51.336307	2026-07-27 07:14:51.336307
1363	1363	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-27 07:14:51.342091	2026-07-27 07:14:51.342091
1364	1364	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-27 07:14:51.360027	2026-07-27 07:14:51.360027
1365	1365	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-27 07:14:51.377868	2026-07-27 07:14:51.377868
1366	1366	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-27 07:14:51.394203	2026-07-27 07:14:51.394203
1367	1367	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-27 07:14:52.509815	2026-07-27 07:14:52.509815
1368	1368	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-27 07:14:52.52767	2026-07-27 07:14:52.52767
1369	1369	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-27 07:14:52.544735	2026-07-27 07:14:52.544735
1370	1370	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-27 07:14:52.56123	2026-07-27 07:14:52.56123
1371	1371	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-27 07:14:52.579477	2026-07-27 07:14:52.579477
1372	1372	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-27 10:12:33.25546	2026-07-27 10:12:33.25546
1373	1373	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-27 10:12:33.263035	2026-07-27 10:12:33.263035
1374	1374	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-27 10:12:33.274156	2026-07-27 10:12:33.274156
1376	1376	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-27 10:12:33.310288	2026-07-27 10:12:33.310288
1377	1377	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-27 10:12:33.334671	2026-07-27 10:12:33.334671
1378	1378	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-27 10:12:33.352159	2026-07-27 10:12:33.352159
1379	1379	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-27 10:12:33.369804	2026-07-27 10:12:33.369804
1380	1380	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-27 10:12:33.376496	2026-07-27 10:12:33.376496
1381	1381	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-27 10:12:33.389334	2026-07-27 10:12:33.389334
1382	1382	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-27 10:12:33.406755	2026-07-27 10:12:33.406755
1383	1383	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-27 10:12:33.422122	2026-07-27 10:12:33.422122
1384	1384	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-27 10:12:34.585056	2026-07-27 10:12:34.585056
1385	1385	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-27 10:12:34.604261	2026-07-27 10:12:34.604261
1386	1386	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-27 10:12:34.620689	2026-07-27 10:12:34.620689
1387	1387	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-27 10:12:34.638973	2026-07-27 10:12:34.638973
1388	1388	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-27 10:12:34.655902	2026-07-27 10:12:34.655902
1389	1389	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-27 10:17:31.94385	2026-07-27 10:17:31.94385
1390	1390	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-27 10:17:31.951307	2026-07-27 10:17:31.951307
1391	1391	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-27 10:17:31.962338	2026-07-27 10:17:31.962338
1393	1393	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-27 10:17:32.000471	2026-07-27 10:17:32.000471
1394	1394	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-27 10:17:32.023428	2026-07-27 10:17:32.023428
1395	1395	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-27 10:17:32.040193	2026-07-27 10:17:32.040193
1396	1396	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-27 10:17:32.058987	2026-07-27 10:17:32.058987
1397	1397	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-27 10:17:32.065933	2026-07-27 10:17:32.065933
1398	1398	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-27 10:17:32.081033	2026-07-27 10:17:32.081033
1399	1399	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-27 10:17:32.100048	2026-07-27 10:17:32.100048
1400	1400	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-27 10:17:32.116928	2026-07-27 10:17:32.116928
1401	1401	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-27 10:17:33.289592	2026-07-27 10:17:33.289592
1402	1402	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-27 10:17:33.310168	2026-07-27 10:17:33.310168
1403	1403	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-27 10:17:33.327002	2026-07-27 10:17:33.327002
1404	1404	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-27 10:17:33.342821	2026-07-27 10:17:33.342821
1405	1405	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-27 10:17:33.358699	2026-07-27 10:17:33.358699
1406	1406	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-27 10:21:49.293776	2026-07-27 10:21:49.293776
1407	1407	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-27 10:21:49.300043	2026-07-27 10:21:49.300043
1408	1408	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-27 10:21:49.310508	2026-07-27 10:21:49.310508
1410	1410	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-27 10:21:49.346495	2026-07-27 10:21:49.346495
1411	1411	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-27 10:21:49.368984	2026-07-27 10:21:49.368984
1412	1412	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-27 10:21:49.385398	2026-07-27 10:21:49.385398
1413	1413	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-27 10:21:49.402378	2026-07-27 10:21:49.402378
1414	1414	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-27 10:21:49.408026	2026-07-27 10:21:49.408026
1415	1415	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-27 10:21:49.420491	2026-07-27 10:21:49.420491
1416	1416	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-27 10:21:49.438	2026-07-27 10:21:49.438
1417	1417	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-27 10:21:49.455955	2026-07-27 10:21:49.455955
1418	1418	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-27 10:21:50.599706	2026-07-27 10:21:50.599706
1419	1419	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-27 10:21:50.619964	2026-07-27 10:21:50.619964
1420	1420	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-27 10:21:50.639665	2026-07-27 10:21:50.639665
1421	1421	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-27 10:21:50.656015	2026-07-27 10:21:50.656015
1422	1422	3	/books/manual_upload.epub	\N	\N	\N	\N	f	f	t	f	t	\N	\N	2026-07-27 10:21:50.673931	2026-07-27 10:21:50.673931
\.


--
-- Data for Name: edition_tags; Type: TABLE DATA; Schema: public; Owner: -
--

COPY public.edition_tags (edition_id, tag_id) FROM stdin;
\.


--
-- Data for Name: editions; Type: TABLE DATA; Schema: public; Owner: -
--

COPY public.editions (id, work_id, isbn, ean, udc, bbk, title, language, publisher, year, city, pages, series, series_number, annotation, source, is_complete, quality, on_shelf, shelf_order, created_at, updated_at, upload_date, cover_path, lower_title, search_vector, uploaded_by) FROM stdin;
780	776	978-5-98062-078-3	\N	\N	\N	Оргуправленческое мышление: идеология, методология, технология	rus		2014	\N	\N	\N	\N	\N	imported	t	good	f	0	2026-07-21 15:44:10.227431	2026-07-21 15:44:10.227431	2026-07-21 15:44:10.227431	\N	оргуправленческое мышление: идеология, методология, технология	\N	1
781	777	\N	\N	\N	\N	KGBT+ (КГБТ+)	rus		2022	\N	\N	\N	\N	\N	imported	t	good	f	0	2026-07-21 15:44:10.290163	2026-07-21 15:44:10.290163	2026-07-21 15:44:10.290163	\N	kgbt+ (кгбт+)	\N	1
782	778	\N	\N	\N	\N	Беседы с Богом. Необычный диалог. Книга 1	rus	София	1995	\N	\N	\N	\N	\N	imported	t	good	f	0	2026-07-21 15:44:10.306207	2026-07-21 15:44:10.306207	2026-07-21 15:44:10.306207	\N	беседы с богом. необычный диалог. книга 1	\N	1
783	779	978-5-6049168-9-6	\N	\N	\N	Жёлтый вождь	rus		1869	\N	\N	\N	\N	\N	imported	t	good	f	0	2026-07-21 15:44:10.335852	2026-07-21 15:44:10.335852	2026-07-21 15:44:10.335852	\N	желтый вождь	\N	1
784	780	\N	\N	\N	\N	Охотники за скальпами	rus		\N	\N	\N	\N	\N	\N	imported	t	good	f	0	2026-07-21 15:44:10.366339	2026-07-21 15:44:10.366339	2026-07-21 15:44:10.366339	\N	охотники за скальпами	\N	1
785	781	978-5-04-193507-8	\N	\N	\N	Путешествие в Элевсин	rus		2023	\N	\N	\N	\N	\N	imported	t	good	f	0	2026-07-21 15:44:10.443562	2026-07-21 15:44:10.443562	2026-07-21 15:44:10.443562	\N	путешествие в элевсин	\N	1
786	782	0869-3951	\N	\N	\N	Фантастический альманах «Завтра».  Выпуск четвертый	rus		\N	\N	\N	\N	\N	\N	imported	t	good	f	0	2026-07-21 15:44:10.52771	2026-07-21 15:44:10.52771	2026-07-21 15:44:10.52771	\N	фантастический альманах «завтра».  выпуск четвертый	\N	1
1406	1398	\N	\N	\N	\N	Test Book Part 1	eng	Self-published	2023	Self-published	0	\N	\N	First test book created via API	manual	t	good	f	0	2026-07-27 10:21:49.293776	2026-07-27 10:21:49.293776	2026-07-27 10:21:49.293776	\N	test book part 1	\N	\N
788	784	978-5-04-123118-7	\N	\N	\N	TRANSHUMANISM INC.	rus		2021	\N	\N	\N	\N	\N	imported	t	good	f	0	2026-07-21 15:44:10.680896	2026-07-21 15:44:10.680896	2026-07-21 15:44:10.680896	\N	transhumanism inc.	\N	1
789	191	\N	\N	\N	\N	Чёрный лебедь. Под знаком непредсказуемости	eng		\N	\N	\N	\N	\N	\N	imported	t	good	f	0	2026-07-21 15:45:10.406087	2026-07-21 15:45:10.406087	2026-07-21 15:45:10.406087	\N	черный лебедь. под знаком непредсказуемости	\N	1
790	785	978-5-4484-8218-2	\N	\N	\N	Пиратский остров; Молодые невольники	rus		1865	\N	\N	\N	\N	\N	imported	t	good	f	0	2026-07-21 15:45:10.815939	2026-07-21 15:45:10.815939	2026-07-21 15:45:10.815939	\N	пиратский остров; молодые невольники	\N	1
791	786	978-5-4484-8313-4	\N	\N	\N	Смертельный выстрел	rus		1873	\N	\N	\N	\N	\N	imported	t	good	f	0	2026-07-21 15:45:11.044353	2026-07-21 15:45:11.044353	2026-07-21 15:45:11.044353	\N	смертельный выстрел	\N	1
792	787	978-5-6049168-0-3	\N	\N	\N	Пропавшая гора	rus		1882	\N	\N	\N	\N	\N	imported	t	good	f	0	2026-07-21 15:45:11.063954	2026-07-21 15:45:11.063954	2026-07-21 15:45:11.063954	\N	пропавшая гора	\N	1
793	788	978-5-6050126-5-8	\N	\N	\N	Пронзенное сердце и другие рассказы	rus		\N	\N	\N	\N	\N	\N	imported	t	good	f	0	2026-07-21 15:45:11.086615	2026-07-21 15:45:11.086615	2026-07-21 15:45:11.086615	\N	пронзенное сердце и другие рассказы	\N	1
794	789	978-5-389-24854-0	\N	\N	\N	Всадник без головы. Морской волчонок	rus		\N	\N	\N	\N	\N	\N	imported	t	good	f	0	2026-07-21 15:45:11.155943	2026-07-21 15:45:11.155943	2026-07-21 15:45:11.155943	\N	всадник без головы. морской волчонок	\N	1
795	790	\N	\N	\N	\N	Мья навыков высоко-эффективных людей: мощные инструменты развития личности	eng		\N	\N	\N	\N	\N	\N	imported	t	good	f	0	2026-07-21 15:45:37.997756	2026-07-21 15:45:37.997756	2026-07-21 15:45:37.997756	\N	мья навыков высоко-эффективных людей: мощные инструменты развития личности	\N	1
796	791	5-7356-0007-9	\N	\N	\N	Отважная охотница	rus		\N	\N	\N	\N	\N	\N	imported	t	good	f	0	2026-07-21 15:45:38.110103	2026-07-21 15:45:38.110103	2026-07-21 15:45:38.110103	\N	отважная охотница	\N	1
797	792	978-5-9533-3644-4	\N	\N	\N	Охота на левиафана	rus		\N	\N	\N	\N	\N	\N	imported	t	good	f	0	2026-07-21 15:45:38.156859	2026-07-21 15:45:38.156859	2026-07-21 15:45:38.156859	\N	охота на левиафана	\N	1
798	793	\N	\N	\N	\N	ИСКУССТВО СНОВИДЕНИЯ.	eng		\N	\N	\N	\N	\N	\N	imported	t	good	f	0	2026-07-21 15:46:12.559763	2026-07-21 15:46:12.559763	2026-07-21 15:46:12.559763	\N	искусство сновидения.	\N	1
802	797	\N	\N	\N	\N	Data Science from Scratch: First Principles with Python	eng		\N	\N	\N	\N	\N	\N	imported	t	good	f	0	2026-07-21 15:46:45.354403	2026-07-21 15:46:45.354403	2026-07-21 15:46:45.354403	\N	data science from scratch: first principles with python	\N	1
803	798	978-5-17-079020-3	\N	\N	\N	Время – деньги!	rus		2013	\N	\N	\N	\N	\N	imported	t	good	f	0	2026-07-21 15:46:45.395766	2026-07-21 15:46:45.395766	2026-07-21 15:46:45.395766	\N	время – деньги!	\N	1
438	434	\N	\N	\N	\N	Test Book Part 1	eng	Self-published	2023	Self-published	0	\N	\N	First test book created via API	manual	t	good	f	0	2026-07-15 15:44:42.120611	2026-07-15 15:44:42.120611	2026-07-15 15:44:42.120611	\N	test book part 1	\N	\N
804	799	\N	\N	\N	\N	Git_для_профессионального_программиста.pdf	eng		\N	\N	\N	\N	\N	\N	imported	t	good	f	0	2026-07-21 15:46:45.536446	2026-07-21 15:46:45.536446	2026-07-21 15:46:45.536446	\N	git_для_профессионального_программиста.pdf	\N	1
805	800	\N	\N	\N	\N	Ускорение: Совершенствование методов хозяйствования	rus		\N	\N	\N	\N	\N	\N	imported	t	good	f	0	2026-07-21 15:46:45.572924	2026-07-21 15:46:45.572924	2026-07-21 15:46:45.572924	\N	ускорение: совершенствование методов хозяйствования	\N	1
1407	1399	\N	\N	\N	\N	Test Book Part 2	eng	Self-published	2024	Self-published	0	\N	\N	Second test book created via API	manual	t	good	f	0	2026-07-27 10:21:49.300043	2026-07-27 10:21:49.300043	2026-07-27 10:21:49.300043	\N	test book part 2	\N	\N
807	802	\N	\N	\N	\N	Тайная Доктрина. Синтез науки, религии и философии Том II. Антропогенезис	eng		\N	\N	\N	\N	\N	\N	imported	t	good	f	0	2026-07-21 15:48:18.549859	2026-07-21 15:48:18.549859	2026-07-21 15:48:18.549859	\N	тайная доктрина. синтез науки, религии и философии том ii. антропогенезис	\N	1
808	803	\N	\N	\N	\N	HPB-TD3.DOC	eng		\N	\N	\N	\N	\N	\N	imported	t	good	f	0	2026-07-21 15:49:18.673356	2026-07-21 15:49:18.673356	2026-07-21 15:49:18.673356	\N	hpb-td3.doc	\N	1
809	804	\N	\N	\N	\N	Текст книги, предоставленный через выделение "Эту книгу хорошо дополняют", является ссылкой на другие издания и не содержит информации о заглавии или авторе самой основной работы.	eng		\N	\N	\N	\N	\N	\N	imported	t	good	f	0	2026-07-21 15:49:38.65411	2026-07-21 15:49:38.65411	2026-07-21 15:49:38.65411	\N	текст книги, предоставленный через выделение "эту книгу хорошо дополняют", является ссылкой на другие издания и не содержит информации о заглавии или авторе самой основной работы.	\N	1
810	805	\N	\N	\N	\N	Так говорил Заратустра	rus		1885	\N	\N	\N	\N	\N	imported	t	good	f	0	2026-07-21 15:49:38.704073	2026-07-21 15:49:38.704073	2026-07-21 15:49:38.704073	\N	так говорил заратустра	\N	1
799	794	\N	\N	\N	\N	Laracasts Tips and Tricks	eng		\N	\N	\N	\N	\N	\N	imported	t	good	f	0	2026-07-21 15:46:29.363865	2026-07-24 19:13:13.44264	2026-07-21 15:46:29.363865	\N	laracasts tips and tricks	\N	1
801	796	\N	\N	\N	\N	Жизнь внутри пузыря. Неформальное руководство менеджера по выживанию в инвестируемом проекте	rus		2007	\N	\N	\N	\N	\N	imported	t	good	f	1	2026-07-21 15:46:29.412558	2026-07-24 19:08:11.694378	2026-07-21 15:46:29.412558	\N	жизнь внутри пузыря. неформальное руководство менеджера по выживанию в инвестируемом проекте	\N	1
800	795	\N	\N	\N	\N	Иуда Искариот	rus		1907	\N	\N	\N	\N	\N	imported	t	good	f	0	2026-07-21 15:46:29.382077	2026-07-25 15:52:53.676455	2026-07-21 15:46:29.382077	\N	иуда искариот	\N	1
811	776	\N	\N	\N	\N	ОРГУПРАВЛЕНЧЕСКОЕ МЫШЛЕНИЕ: идеология, методология, технология	eng		\N	\N	\N	\N	\N	\N	imported	t	good	f	0	2026-07-21 15:50:20.00399	2026-07-21 15:50:20.00399	2026-07-21 15:50:20.00399	\N	оргуправленческое мышление: идеология, методология, технология	\N	1
812	806	\N	\N	\N	\N	Алмазная Сутра	eng		\N	\N	\N	\N	\N	\N	imported	t	good	f	0	2026-07-21 15:50:20.035534	2026-07-21 15:50:20.035534	2026-07-21 15:50:20.035534	\N	алмазная сутра	\N	1
813	807	\N	\N	\N	\N	Антикризиская программа	eng		2009	\N	\N	\N	\N	\N	imported	t	good	f	0	2026-07-21 15:50:20.050556	2026-07-21 15:50:20.050556	2026-07-21 15:50:20.050556	\N	антикризиская программа	\N	1
814	808	5-11-000807-8	\N	\N	\N	Белый Лотос	eng		2009	\N	\N	\N	\N	\N	imported	t	good	f	0	2026-07-21 15:50:20.093723	2026-07-21 15:50:20.093723	2026-07-21 15:50:20.093723	\N	белый лотос	\N	1
815	809	\N	\N	\N	\N	Близость. Доверие к себе и другому.	eng		2009	\N	\N	\N	\N	\N	imported	t	good	f	0	2026-07-21 15:50:20.11649	2026-07-21 15:50:20.11649	2026-07-21 15:50:20.11649	\N	близость. доверие к себе и другому.	\N	1
816	810	\N	\N	\N	\N	Будда: Пустота Сердца	eng		\N	\N	\N	\N	\N	\N	imported	t	good	f	0	2026-07-21 15:50:20.140947	2026-07-21 15:50:20.140947	2026-07-21 15:50:20.140947	\N	будда: пустота сердца	\N	1
817	811	5-9550-0239-1	\N	\N	\N	Горчичное зерно. Комментарии к пятому Евангелию от св. Фомы	eng		\N	\N	\N	\N	\N	\N	imported	t	good	f	0	2026-07-21 15:50:20.202047	2026-07-21 15:50:20.202047	2026-07-21 15:50:20.202047	\N	горчичное зерно. комментарии к пятому евангелию от св. фомы	\N	1
818	812	\N	\N	\N	\N	Гусь снаружи	eng		2009	\N	\N	\N	\N	\N	imported	t	good	f	0	2026-07-21 15:50:20.232632	2026-07-21 15:50:20.232632	2026-07-21 15:50:20.232632	\N	гусь снаружи	\N	1
819	813	978-91250-273-6	\N	\N	\N	В поисках Чудесного. Чакры, Кундалини и семь тел	eng		\N	\N	\N	\N	\N	\N	imported	t	good	f	0	2026-07-21 15:50:20.275048	2026-07-21 15:50:20.275048	2026-07-21 15:50:20.275048	\N	в поисках чудесного. чакры, кундалини и семь тел	\N	1
820	814	\N	\N	\N	\N	Алмазные россыпи	eng		2009	\N	\N	\N	\N	\N	imported	t	good	f	0	2026-07-21 15:50:20.286397	2026-07-21 15:50:20.286397	2026-07-21 15:50:20.286397	\N	алмазные россыпи	\N	1
821	815	\N	\N	\N	\N	Руководство к своду знаний по управлению проектами (Руководство PMBOK). Редакция 2000 года	eng		\N	\N	\N	\N	\N	\N	imported	t	good	f	0	2026-07-21 15:51:00.123321	2026-07-21 15:51:00.123321	2026-07-21 15:51:00.123321	\N	руководство к своду знаний по управлению проектами (руководство pmbok). редакция 2000 года	\N	1
822	816	978-5-699-91778-5	\N	\N	\N	Лампа Мафусаила, или Крайняя битва чекистов с масонами	rus		2016	\N	\N	\N	\N	\N	imported	t	good	f	0	2026-07-21 15:51:00.182008	2026-07-21 15:51:00.182008	2026-07-21 15:51:00.182008	\N	лампа мафусаила, или крайняя битва чекистов с масонами	\N	1
823	817	978-5-04-106222-4	\N	\N	\N	Искусство легких касаний	rus		\N	\N	\N	\N	\N	\N	imported	t	good	f	0	2026-07-21 15:51:00.258903	2026-07-21 15:51:00.258903	2026-07-21 15:51:00.258903	\N	искусство легких касаний	\N	1
824	818	978-5-532-02805-0	\N	\N	\N	Непобедимое солнце	rus		2020	\N	\N	\N	\N	\N	imported	t	good	f	0	2026-07-21 15:51:00.320256	2026-07-21 15:51:00.320256	2026-07-21 15:51:00.320256	\N	непобедимое солнце	\N	1
825	819	978-5-04-089394-2	\N	\N	\N	iPhuck 10	rus		2017	\N	\N	\N	\N	\N	imported	t	good	f	0	2026-07-21 15:51:00.380568	2026-07-21 15:51:00.380568	2026-07-21 15:51:00.380568	\N	iphuck 10	\N	1
826	820	978-5-17-133459-8	\N	\N	\N	Шум. Несовершенство человеческих суждений	rus		2021	\N	\N	\N	\N	\N	imported	t	good	f	0	2026-07-21 15:51:00.455477	2026-07-21 15:51:00.455477	2026-07-21 15:51:00.455477	\N	шум. несовершенство человеческих суждений	\N	1
827	821	978-5-699-28900-4	\N	\N	\N	Анна Каренина	rus		1878	\N	\N	\N	\N	\N	imported	t	good	f	0	2026-07-21 15:51:00.588068	2026-07-21 15:51:00.588068	2026-07-21 15:51:00.588068	\N	анна каренина	\N	1
828	822	978-5-04-241368-1	\N	\N	\N	Возвращение Синей Бороды	rus		2026	\N	\N	\N	\N	\N	imported	t	good	f	0	2026-07-21 15:51:00.640797	2026-07-21 15:51:00.640797	2026-07-21 15:51:00.640797	\N	возвращение синей бороды	\N	1
830	824	\N	\N	\N	\N	Жизнь-в-сновидении (Посвящение в мир магов)	eng		\N	\N	\N	\N	\N	\N	imported	t	good	f	0	2026-07-21 15:51:39.683694	2026-07-21 15:51:39.683694	2026-07-21 15:51:39.683694	\N	жизнь-в-сновидении (посвящение в мир магов)	\N	1
831	825	978-5-9614-4352-3	\N	\N	\N	Договориться можно обо всем	rus		\N	\N	\N	\N	\N	\N	imported	t	good	f	0	2026-07-21 15:51:39.729419	2026-07-21 15:51:39.729419	2026-07-21 15:51:39.729419	\N	договориться можно обо всем	\N	1
832	826	\N	\N	\N	\N	Инвестируй в Себя: Разбуди в себе исполина	eng		\N	\N	\N	\N	\N	\N	imported	t	good	f	0	2026-07-21 15:52:16.550161	2026-07-21 15:52:16.550161	2026-07-21 15:52:16.550161	\N	инвестируй в себя: разбуди в себе исполина	\N	1
833	827	\N	\N	\N	\N	Бизнес в стиле фанк: Капитал пляшет под дудку таланта (отрывки из книги)	eng		\N	\N	\N	\N	\N	\N	imported	t	good	f	0	2026-07-21 15:53:01.242092	2026-07-21 15:53:01.242092	2026-07-21 15:53:01.242092	\N	бизнес в стиле фанк: капитал пляшет под дудку таланта (отрывки из книги)	\N	1
835	819	\N	\N	\N	\N	iPhuck 10	rus		2017	\N	\N	\N	\N	\N	imported	t	good	f	0	2026-07-21 15:53:48.105012	2026-07-21 15:53:48.105012	2026-07-21 15:53:48.105012	\N	iphuck 10	\N	1
836	829	\N	\N	\N	\N	isis1.zip	eng		\N	\N	\N	\N	\N	\N	imported	t	good	f	0	2026-07-21 15:54:48.286164	2026-07-21 15:54:48.286164	2026-07-21 15:54:48.286164	\N	isis1.zip	\N	1
837	830	\N	\N	\N	\N	Путешествие к центру Земли	eng		\N	\N	\N	\N	\N	\N	imported	t	good	f	0	2026-07-21 15:55:27.376747	2026-07-21 15:55:27.376747	2026-07-21 15:55:27.376747	\N	путешествие к центру земли	\N	1
439	435	\N	\N	\N	\N	Test Book Part 2	eng	Self-published	2024	Self-published	0	\N	\N	Second test book created via API	manual	t	good	f	0	2026-07-15 15:44:42.126907	2026-07-15 15:44:42.126907	2026-07-15 15:44:42.126907	\N	test book part 2	\N	\N
1408	1400	\N	\N	\N	\N	Updated Book Title	eng	Self-published	2022	Self-published	0	\N	\N	Updated description	manual	t	good	f	0	2026-07-27 10:21:49.310508	2026-07-27 10:21:49.317247	2026-07-27 10:21:49.310508	\N	updated book title	\N	\N
839	832	\N	\N	\N	\N	Магический переход. Путь женщины-воина	eng		\N	\N	\N	\N	\N	\N	imported	t	good	f	0	2026-07-21 15:56:12.463689	2026-07-21 15:56:12.463689	2026-07-21 15:56:12.463689	\N	магический переход. путь женщины-воина	\N	1
840	833	978-5-17-041678-3	\N	\N	\N	Понедельник начинается в субботу	rus		1964	\N	\N	\N	\N	\N	imported	t	good	f	0	2026-07-21 15:56:12.504626	2026-07-21 15:56:12.504626	2026-07-21 15:56:12.504626	\N	понедельник начинается в субботу	\N	1
834	828	\N	\N	\N	\N	Тайная доктрина. Синтез науки, религии и философии. Том I. Космогенезис	eng		\N	\N	\N	\N	\N	\N	imported	t	good	f	0	2026-07-21 15:53:48.025576	2026-07-25 15:58:07.12564	2026-07-21 15:53:48.025576	\N	тайная доктрина. синтез науки, религии и философии. том i. космогенезис	\N	1
444	440	\N	\N	\N	\N	Book Without ISBN	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-15 15:44:42.210132	2026-07-15 15:44:42.210132	2026-07-15 15:44:42.210132	\N	book without isbn	\N	\N
841	834	\N	\N	\N	\N	SHABONO A True Adventure in the Remote and Magical Heart of the South American Jungle	eng		\N	\N	\N	\N	\N	\N	imported	t	good	f	1	2026-07-21 15:57:02.278284	2026-07-24 19:14:02.7552	2026-07-21 15:57:02.278284	\N	shabono a true adventure in the remote and magical heart of the south american jungle	\N	1
842	835	\N	\N	\N	\N	Магические пассы. Практическая мудрость шаманов древней Мексики	eng		\N	\N	\N	\N	\N	\N	imported	t	good	f	0	2026-07-21 15:57:42.098163	2026-07-21 15:57:42.098163	2026-07-21 15:57:42.098163	\N	магические пассы. практическая мудрость шаманов древней мексики	\N	1
843	836	\N	\N	\N	\N	Путь воина: Два года с доктором Хуаном Матусом - Сон ведьмы.	eng		\N	\N	\N	\N	\N	\N	imported	t	good	f	0	2026-07-21 15:58:23.448196	2026-07-21 15:58:23.448196	2026-07-21 15:58:23.448196	\N	путь воина: два года с доктором хуаном матусом - сон ведьмы.	\N	1
846	839	\N	\N	\N	\N	Рассказы Вельзевула своему внуку; Объективно-беспристрастная критика жизни людей; Всё и Вся (в трех сериях)	eng		\N	\N	\N	\N	\N	\N	imported	t	good	f	0	2026-07-21 15:59:47.799375	2026-07-21 15:59:47.799375	2026-07-21 15:59:47.799375	\N	рассказы вельзевула своему внуку; объективно-беспристрастная критика жизни людей; все и вся (в трех сериях)	\N	1
847	840	\N	\N	\N	\N	Шестое и последнее пребывание Вельзевула на поверхности Нашей Земли (часть из цикла романов "Архиепископ Плетенецкий")	eng		\N	\N	\N	\N	\N	\N	imported	t	good	f	0	2026-07-21 16:00:29.791841	2026-07-21 16:00:29.791841	2026-07-21 16:00:29.791841	\N	шестое и последнее пребывание вельзевула на поверхности нашей земли (часть из цикла романов "архиепископ плетенецкий")	\N	1
848	841	\N	\N	\N	\N	Свет на Пути в Гору Святых, Великий Учитель Гаутам и Друзья, Книга 1	eng		\N	\N	\N	\N	\N	\N	imported	t	good	f	0	2026-07-21 16:01:07.795816	2026-07-21 16:01:07.795816	2026-07-21 16:01:07.795816	\N	свет на пути в гору святых, великий учитель гаутам и друзья, книга 1	\N	1
849	842	\N	\N	\N	\N	Голос безмолвия, или Два пути; Семь врат (из сокровенных индусских писаний)	eng		\N	\N	\N	\N	\N	\N	imported	t	good	f	0	2026-07-21 16:02:00.708856	2026-07-21 16:02:00.708856	2026-07-21 16:02:00.708856	\N	голос безмолвия, или два пути; семь врат (из сокровенных индусских писаний)	\N	1
850	843	\N	\N	\N	\N	Беседы с учениками	rus		\N	\N	\N	\N	\N	\N	imported	t	good	f	0	2026-07-21 16:02:00.721419	2026-07-21 16:02:00.721419	2026-07-21 16:02:00.721419	\N	беседы с учениками	\N	1
851	844	5-85990-050-3	\N	\N	\N	Все и вся	rus		\N	\N	\N	\N	\N	\N	imported	t	good	f	0	2026-07-21 16:02:00.815672	2026-07-21 16:02:00.815672	2026-07-21 16:02:00.815672	\N	все и вся	\N	1
853	845	\N	\N	\N	\N	Встречи с замечательными людьми	rus		2008	\N	\N	\N	\N	\N	imported	t	good	f	0	2026-07-21 16:02:00.906156	2026-07-21 16:02:00.906156	2026-07-21 16:02:00.906156	\N	встречи с замечательными людьми	\N	1
855	847	\N	\N	\N	\N	Закономерное разнообразие проявлений человеческой индивидуальности	rus		\N	\N	\N	\N	\N	\N	imported	t	good	f	0	2026-07-21 16:02:00.945292	2026-07-21 16:02:00.945292	2026-07-21 16:02:00.945292	\N	закономерное разнообразие проявлений человеческой индивидуальности	\N	1
856	848	\N	\N	\N	\N	Последний час жизни	rus		\N	\N	\N	\N	\N	\N	imported	t	good	f	0	2026-07-21 16:02:00.952545	2026-07-21 16:02:00.952545	2026-07-21 16:02:00.952545	\N	последний час жизни	\N	1
857	849	\N	\N	\N	\N	Человек - это многосложное существо	rus		2008	\N	\N	\N	\N	\N	imported	t	good	f	0	2026-07-21 16:02:00.975786	2026-07-21 16:02:00.975786	2026-07-21 16:02:00.975786	\N	человек - это многосложное существо	\N	1
858	850	\N	\N	\N	\N	Эссе и размышления о Человеке и его Учении	rus		\N	\N	\N	\N	\N	\N	imported	t	good	f	0	2026-07-21 16:02:00.981325	2026-07-21 16:02:00.981325	2026-07-21 16:02:00.981325	\N	эссе и размышления о человеке и его учении	\N	1
859	851	\N	\N	\N	\N	Дао дэ цзин	rus		\N	\N	\N	\N	\N	\N	imported	t	good	f	0	2026-07-21 16:02:01.050855	2026-07-21 16:02:01.050855	2026-07-21 16:02:01.050855	\N	дао дэ цзин	\N	1
860	852	9785001464273	\N	\N	\N	Путь джедая	rus	Манн, Иванов и Фербер	2020	\N	\N	\N	\N	\N	imported	t	good	f	0	2026-07-21 16:02:01.162583	2026-07-21 16:02:01.162583	2026-07-21 16:02:01.162583	\N	путь джедая	\N	1
861	853	\N	\N	\N	\N	Название и автор указанного текста не определены, так как представлены дополнительные книги.	eng		\N	\N	\N	\N	\N	\N	imported	t	good	f	0	2026-07-21 16:02:13.116777	2026-07-21 16:02:13.116777	2026-07-21 16:02:13.116777	\N	название и автор указанного текста не определены, так как представлены дополнительные книги.	\N	1
862	854	\N	\N	\N	\N	Учение дона Хуана: путь знания индейцев яки	rus		\N	\N	\N	\N	\N	\N	imported	t	good	f	0	2026-07-21 16:02:13.150713	2026-07-21 16:02:13.150713	2026-07-21 16:02:13.150713	\N	учение дона хуана: путь знания индейцев яки	\N	1
863	855	\N	\N	\N	\N	ОТДЕЛЕННАЯ РЕАЛЬНОСТЬ (Книга 2)	eng		\N	\N	\N	\N	\N	\N	imported	t	good	f	0	2026-07-21 16:02:51.827047	2026-07-21 16:02:51.827047	2026-07-21 16:02:51.827047	\N	отделенная реальность (книга 2)	\N	1
864	856	\N	\N	\N	\N	Путешествие в Икстлан. Путь к знанию и силе йоги шаманизма мексиканских индейцев	eng		\N	\N	\N	\N	\N	\N	imported	t	good	f	0	2026-07-21 16:03:37.246577	2026-07-21 16:03:37.246577	2026-07-21 16:03:37.246577	\N	путешествие в икстлан. путь к знанию и силе йоги шаманизма мексиканских индейцев	\N	1
865	857	\N	\N	\N	\N	Сказки о силе: книга 4	eng		\N	\N	\N	\N	\N	\N	imported	t	good	f	0	2026-07-21 16:04:13.680014	2026-07-21 16:04:13.680014	2026-07-21 16:04:13.680014	\N	сказки о силе: книга 4	\N	1
866	858	\N	\N	\N	\N	Второе кольцо силы. Перекрестье жизней  \n(Note: The book mentioned is part 5 of a larger work, the full title may vary in different editions.)	eng		\N	\N	\N	\N	\N	\N	imported	t	good	f	0	2026-07-21 16:05:01.023192	2026-07-21 16:05:01.023192	2026-07-21 16:05:01.023192	\N	второе кольцо силы. перекрестье жизней  \n(note: the book mentioned is part 5 of a larger work, the full title may vary in different editions.)	\N	1
867	859	\N	\N	\N	\N	Книга 6. Дар Орла	eng		\N	\N	\N	\N	\N	\N	imported	t	good	f	0	2026-07-21 16:05:34.764247	2026-07-21 16:05:34.764247	2026-07-21 16:05:34.764247	\N	книга 6. дар орла	\N	1
883	875	\N	\N	\N	\N	Book With ISBN Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-21 16:09:01.060855	2026-07-21 16:09:01.060855	2026-07-21 16:09:01.060855	\N	book with isbn test	\N	\N
233	229	\N	\N	\N	\N	Add Author Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-12 12:18:32.826986	2026-07-23 13:17:34.632757	2026-07-12 12:18:32.826986	\N	add author test	\N	\N
845	838	\N	\N	\N	\N	НФ: Альманах научной фантастики 35 (1991)	rus		\N	\N	\N	\N	\N	\N	imported	t	good	f	0	2026-07-21 15:59:05.914317	2026-07-24 15:36:14.635592	2026-07-21 15:59:05.914317	\N	нф: альманах научной фантастики 35 (1991)	\N	1
852	845	\N	\N	\N	\N	Встречи с замечательными людьми	rus		2008	\N	\N	\N	\N	\N	imported	t	good	f	1	2026-07-21 16:02:00.862362	2026-07-24 13:53:50.719935	2026-07-21 16:02:00.862362	\N	встречи с замечательными людьми	\N	1
844	837	\N	\N	\N	\N	Взгляды из реального мира. Записи бесед и лекций Гурджиева. Самонаблюдение.	eng		\N	\N	\N	\N	\N	\N	imported	t	good	f	1	2026-07-21 15:59:05.869285	2026-07-24 15:43:05.573516	2026-07-21 15:59:05.869285	\N	взгляды из реального мира. записи бесед и лекций гурджиева. самонаблюдение.	\N	1
884	876	\N	\N	\N	\N	Book Without ISBN	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-21 16:09:01.078846	2026-07-21 16:09:01.078846	2026-07-21 16:09:01.078846	\N	book without isbn	\N	\N
102	99	5-699-12257-5	\N	\N	\N	Who by fire	rus		\N	\N	\N	\N	\N	\N	imported	t	good	f	0	2026-07-10 17:16:16.320947	2026-07-10 17:16:16.320947	2026-07-10 17:16:16.320947	\N	who by fire	\N	\N
104	101	\N	\N	\N	\N	Акико	rus		\N	\N	\N	\N	\N	\N	imported	t	good	f	0	2026-07-10 17:16:16.376014	2026-07-10 17:16:16.376014	2026-07-10 17:16:16.376014	\N	акико	\N	\N
105	102	5-699-19085-6	\N	\N	\N	Ампир «В»	rus		2006	\N	\N	\N	\N	\N	imported	t	good	f	0	2026-07-10 17:16:16.419499	2026-07-10 17:16:16.419499	2026-07-10 17:16:16.419499	\N	ампир «в»	\N	\N
106	103	978-5-699-46291-9	\N	\N	\N	Ананасная вода для прекрасной дамы	rus		\N	\N	\N	\N	\N	\N	imported	t	good	f	0	2026-07-10 17:16:16.456042	2026-07-10 17:16:16.456042	2026-07-10 17:16:16.456042	\N	ананасная вода для прекрасной дамы	\N	\N
107	104	\N	\N	\N	\N	Бубен верхнего мира	rus		\N	\N	\N	\N	\N	\N	imported	t	good	f	0	2026-07-10 17:16:16.46316	2026-07-10 17:16:16.46316	2026-07-10 17:16:16.46316	\N	бубен верхнего мира	\N	\N
108	105	\N	\N	\N	\N	Бубен нижнего мира	rus		\N	\N	\N	\N	\N	\N	imported	t	good	f	0	2026-07-10 17:16:16.469526	2026-07-10 17:16:16.469526	2026-07-10 17:16:16.469526	\N	бубен нижнего мира	\N	\N
109	106	978-5-699-63446-0	\N	\N	\N	Бэтман Аполло	rus		2013	\N	\N	\N	\N	\N	imported	t	good	f	0	2026-07-10 17:16:16.528354	2026-07-10 17:16:16.528354	2026-07-10 17:16:16.528354	\N	бэтман аполло	\N	\N
110	107	\N	\N	\N	\N	Вести из Непала	rus		\N	\N	\N	\N	\N	\N	imported	t	good	f	0	2026-07-10 17:16:16.537958	2026-07-10 17:16:16.537958	2026-07-10 17:16:16.537958	\N	вести из непала	\N	\N
111	108	\N	\N	\N	\N	Виктор Пелевин спрашивает PRов	rus		\N	\N	\N	\N	\N	\N	imported	t	good	f	0	2026-07-10 17:16:16.543519	2026-07-10 17:16:16.543519	2026-07-10 17:16:16.543519	\N	виктор пелевин спрашивает prов	\N	\N
112	109	\N	\N	\N	\N	Водонапорная башня	rus		\N	\N	\N	\N	\N	\N	imported	t	good	f	0	2026-07-10 17:16:16.549714	2026-07-10 17:16:16.549714	2026-07-10 17:16:16.549714	\N	водонапорная башня	\N	\N
113	110	\N	\N	\N	\N	Все рассказы (Сборник)	rus		\N	\N	\N	\N	\N	\N	imported	t	good	f	0	2026-07-10 17:16:16.616765	2026-07-10 17:16:16.616765	2026-07-10 17:16:16.616765	\N	все рассказы (сборник)	\N	\N
114	111	\N	\N	\N	\N	Встроенный напоминатель	rus		\N	\N	\N	\N	\N	\N	imported	t	good	f	0	2026-07-10 17:16:16.623366	2026-07-10 17:16:16.623366	2026-07-10 17:16:16.623366	\N	встроенный напоминатель	\N	\N
115	112	\N	\N	\N	\N	ГКЧП как тетраграмматон	rus		\N	\N	\N	\N	\N	\N	imported	t	good	f	0	2026-07-10 17:16:16.62855	2026-07-10 17:16:16.62855	2026-07-10 17:16:16.62855	\N	гкчп как тетраграмматон	\N	\N
116	113	\N	\N	\N	\N	Гадание на рунах или рунический оракул Ральфа Блума	rus		\N	\N	\N	\N	\N	\N	imported	t	good	f	0	2026-07-10 17:16:16.636659	2026-07-10 17:16:16.636659	2026-07-10 17:16:16.636659	\N	гадание на рунах или рунический оракул ральфа блума	\N	\N
117	114	\N	\N	\N	\N	Греческий вариант	rus		\N	\N	\N	\N	\N	\N	imported	t	good	f	0	2026-07-10 17:16:16.642782	2026-07-10 17:16:16.642782	2026-07-10 17:16:16.642782	\N	греческий вариант	\N	\N
118	115	978-5-699-82544-8	\N	\N	\N	ДПП (НН) (сборник)	rus		2003	\N	\N	\N	\N	\N	imported	t	good	f	0	2026-07-10 17:16:16.722467	2026-07-10 17:16:16.722467	2026-07-10 17:16:16.722467	\N	дпп (нн) (сборник)	\N	\N
119	116	\N	\N	\N	\N	Девятый сон Веры Павловны	rus		\N	\N	\N	\N	\N	\N	imported	t	good	f	0	2026-07-10 17:16:16.731626	2026-07-10 17:16:16.731626	2026-07-10 17:16:16.731626	\N	девятый сон веры павловны	\N	\N
120	117	\N	\N	\N	\N	День бульдозериста	rus		\N	\N	\N	\N	\N	\N	imported	t	good	f	0	2026-07-10 17:16:16.740215	2026-07-10 17:16:16.740215	2026-07-10 17:16:16.740215	\N	день бульдозериста	\N	\N
121	118	\N	\N	\N	\N	Джон Фаулз и трагедия русского либерализма	rus		\N	\N	\N	\N	\N	\N	imported	t	good	f	0	2026-07-10 17:16:16.746196	2026-07-10 17:16:16.746196	2026-07-10 17:16:16.746196	\N	джон фаулз и трагедия русского либерализма	\N	\N
122	119	5-699-03491-9	\N	\N	\N	Диалектика Переходного Периода Из Ниоткуда В Никуда	rus		2003	\N	\N	\N	\N	\N	imported	t	good	f	0	2026-07-10 17:16:16.788	2026-07-10 17:16:16.788	2026-07-10 17:16:16.788	\N	диалектика переходного периода из ниоткуда в никуда	\N	\N
123	120	5-264-00863-9	\N	\N	\N	Желтая стрела	rus		\N	\N	\N	\N	\N	\N	imported	t	good	f	0	2026-07-10 17:16:16.799785	2026-07-10 17:16:16.799785	2026-07-10 17:16:16.799785	\N	желтая стрела	\N	\N
124	121	\N	\N	\N	\N	Жизнь и приключения сарая номер XII	rus		\N	\N	\N	\N	\N	\N	imported	t	good	f	0	2026-07-10 17:16:16.805983	2026-07-10 17:16:16.805983	2026-07-10 17:16:16.805983	\N	жизнь и приключения сарая номер xii	\N	\N
125	122	5-264-00746-1	\N	\N	\N	Жизнь насекомых	rus		1993	\N	\N	\N	\N	\N	imported	t	good	f	0	2026-07-10 17:16:16.83182	2026-07-10 17:16:16.83182	2026-07-10 17:16:16.83182	\N	жизнь насекомых	\N	\N
126	123	978-5-699-3053	\N	\N	\N	Зал поющих кариатид	rus		\N	\N	\N	\N	\N	\N	imported	t	good	f	0	2026-07-10 17:16:16.84748	2026-07-10 17:16:16.84748	2026-07-10 17:16:16.84748	\N	зал поющих кариатид	\N	\N
127	124	978-5-699-22664-1	\N	\N	\N	Запись о поиске ветра	rus		2003	\N	\N	\N	\N	\N	imported	t	good	f	0	2026-07-10 17:16:16.858031	2026-07-10 17:16:16.858031	2026-07-10 17:16:16.858031	\N	запись о поиске ветра	\N	\N
128	125	\N	\N	\N	\N	Затворник и Шестипалый	rus		\N	\N	\N	\N	\N	\N	imported	t	good	f	0	2026-07-10 17:16:16.868426	2026-07-10 17:16:16.868426	2026-07-10 17:16:16.868426	\N	затворник и шестипалый	\N	\N
129	126	\N	\N	\N	\N	Зигмунд в кафе	rus		\N	\N	\N	\N	\N	\N	imported	t	good	f	0	2026-07-10 17:16:16.874251	2026-07-10 17:16:16.874251	2026-07-10 17:16:16.874251	\N	зигмунд в кафе	\N	\N
130	127	\N	\N	\N	\N	Зомбификация. Опыт сравнительной антропологии	rus		\N	\N	\N	\N	\N	\N	imported	t	good	f	0	2026-07-10 17:16:16.883165	2026-07-10 17:16:16.883165	2026-07-10 17:16:16.883165	\N	зомбификация. опыт сравнительной антропологии	\N	\N
131	128	\N	\N	\N	\N	Иван Кублаханов	rus		\N	\N	\N	\N	\N	\N	imported	t	good	f	0	2026-07-10 17:16:16.890637	2026-07-10 17:16:16.890637	2026-07-10 17:16:16.890637	\N	иван кублаханов	\N	\N
132	129	\N	\N	\N	\N	Икстлан – Петушки	rus		\N	\N	\N	\N	\N	\N	imported	t	good	f	0	2026-07-10 17:16:16.896876	2026-07-10 17:16:16.896876	2026-07-10 17:16:16.896876	\N	икстлан – петушки	\N	\N
133	130	\N	\N	\N	\N	Имена олигархов на карте Родины	rus		\N	\N	\N	\N	\N	\N	imported	t	good	f	0	2026-07-10 17:16:16.903472	2026-07-10 17:16:16.903472	2026-07-10 17:16:16.903472	\N	имена олигархов на карте родины	\N	\N
134	131	\N	\N	\N	\N	Интервью с Виктором Пелевиным (2)	rus		\N	\N	\N	\N	\N	\N	imported	t	good	f	0	2026-07-10 17:16:16.907737	2026-07-10 17:16:16.907737	2026-07-10 17:16:16.907737	\N	интервью с виктором пелевиным (2)	\N	\N
135	132	\N	\N	\N	\N	Интервью с Виктором Пелевиным	rus		\N	\N	\N	\N	\N	\N	imported	t	good	f	0	2026-07-10 17:16:16.911941	2026-07-10 17:16:16.911941	2026-07-10 17:16:16.911941	\N	интервью с виктором пелевиным	\N	\N
136	133	\N	\N	\N	\N	Код Мира	rus		\N	\N	\N	\N	\N	\N	imported	t	good	f	0	2026-07-10 17:16:16.916201	2026-07-10 17:16:16.916201	2026-07-10 17:16:16.916201	\N	код мира	\N	\N
137	134	\N	\N	\N	\N	Колдун Игнат и люди (сборник)	rus		2007	\N	\N	\N	\N	\N	imported	t	good	f	0	2026-07-10 17:16:16.932095	2026-07-10 17:16:16.932095	2026-07-10 17:16:16.932095	\N	колдун игнат и люди (сборник)	\N	\N
138	135	\N	\N	\N	\N	Колдун Игнат и люди	rus		\N	\N	\N	\N	\N	\N	imported	t	good	f	0	2026-07-10 17:16:16.936702	2026-07-10 17:16:16.936702	2026-07-10 17:16:16.936702	\N	колдун игнат и люди	\N	\N
139	136	\N	\N	\N	\N	Кормление крокодила Хуфу	rus		\N	\N	\N	\N	\N	\N	imported	t	good	f	0	2026-07-10 17:16:16.942997	2026-07-10 17:16:16.942997	2026-07-10 17:16:16.942997	\N	кормление крокодила хуфу	\N	\N
140	137	\N	\N	\N	\N	Краткая история пэйнтбола в Москве	rus		\N	\N	\N	\N	\N	\N	imported	t	good	f	0	2026-07-10 17:16:16.950425	2026-07-10 17:16:16.950425	2026-07-10 17:16:16.950425	\N	краткая история пэйнтбола в москве	\N	\N
141	138	\N	\N	\N	\N	Луноход	rus		\N	\N	\N	\N	\N	\N	imported	t	good	f	0	2026-07-10 17:16:16.9567	2026-07-10 17:16:16.9567	2026-07-10 17:16:16.9567	\N	луноход	\N	\N
142	139	978-5-699-75467-0	\N	\N	\N	Любовь к трем цукербринам	rus		2014	\N	\N	\N	\N	\N	imported	t	good	f	0	2026-07-10 17:16:16.984013	2026-07-10 17:16:16.984013	2026-07-10 17:16:16.984013	\N	любовь к трем цукербринам	\N	\N
103	100	978-5-699-37515-8	\N	\N	\N	t	rus		2009	\N	\N	\N	\N	\N	imported	t	good	f	0	2026-07-10 17:16:16.368714	2026-07-24 15:54:39.01865	2026-07-10 17:16:16.368714	\N	t	\N	\N
143	140	\N	\N	\N	\N	Македонская критика французской мысли (Сборник)	rus		\N	\N	\N	\N	\N	\N	imported	t	good	f	0	2026-07-10 17:16:17.000042	2026-07-10 17:16:17.000042	2026-07-10 17:16:17.000042	\N	македонская критика французской мысли (сборник)	\N	\N
144	141	\N	\N	\N	\N	Мардонги	rus		\N	\N	\N	\N	\N	\N	imported	t	good	f	0	2026-07-10 17:16:17.005258	2026-07-10 17:16:17.005258	2026-07-10 17:16:17.005258	\N	мардонги	\N	\N
145	142	\N	\N	\N	\N	Миттельшпиль	rus		\N	\N	\N	\N	\N	\N	imported	t	good	f	0	2026-07-10 17:16:17.013182	2026-07-10 17:16:17.013182	2026-07-10 17:16:17.013182	\N	миттельшпиль	\N	\N
146	143	\N	\N	\N	\N	Мой мескалитовый трип	rus		2002	\N	\N	\N	\N	\N	imported	t	good	f	0	2026-07-10 17:16:17.017912	2026-07-10 17:16:17.017912	2026-07-10 17:16:17.017912	\N	мой мескалитовый трип	\N	\N
147	144	\N	\N	\N	\N	Мост, который я хотел перейти	rus		\N	\N	\N	\N	\N	\N	imported	t	good	f	0	2026-07-10 17:16:17.02192	2026-07-10 17:16:17.02192	2026-07-10 17:16:17.02192	\N	мост, который я хотел перейти	\N	\N
148	145	\N	\N	\N	\N	Музыка со столба	rus		\N	\N	\N	\N	\N	\N	imported	t	good	f	0	2026-07-10 17:16:17.027413	2026-07-10 17:16:17.027413	2026-07-10 17:16:17.027413	\N	музыка со столба	\N	\N
149	146	\N	\N	\N	\N	Нижняя тундра	rus		\N	\N	\N	\N	\N	\N	imported	t	good	f	0	2026-07-10 17:16:17.033701	2026-07-10 17:16:17.033701	2026-07-10 17:16:17.033701	\N	нижняя тундра	\N	\N
150	147	\N	\N	\N	\N	Ника	rus		\N	\N	\N	\N	\N	\N	imported	t	good	f	0	2026-07-10 17:16:17.040046	2026-07-10 17:16:17.040046	2026-07-10 17:16:17.040046	\N	ника	\N	\N
151	148	5-264-00740-3	\N	\N	\N	Омон Ра	rus		1991	\N	\N	\N	\N	\N	imported	t	good	f	0	2026-07-10 17:16:17.058808	2026-07-10 17:16:17.058808	2026-07-10 17:16:17.058808	\N	омон ра	\N	\N
152	149	\N	\N	\N	\N	Онтология детства	rus		\N	\N	\N	\N	\N	\N	imported	t	good	f	0	2026-07-10 17:16:17.064766	2026-07-10 17:16:17.064766	2026-07-10 17:16:17.064766	\N	онтология детства	\N	\N
153	150	\N	\N	\N	\N	Оружие возмездия	rus		\N	\N	\N	\N	\N	\N	imported	t	good	f	0	2026-07-10 17:16:17.071346	2026-07-10 17:16:17.071346	2026-07-10 17:16:17.071346	\N	оружие возмездия	\N	\N
154	151	\N	\N	\N	\N	Откровение Крегера	rus		\N	\N	\N	\N	\N	\N	imported	t	good	f	0	2026-07-10 17:16:17.076533	2026-07-10 17:16:17.076533	2026-07-10 17:16:17.076533	\N	откровение крегера	\N	\N
155	152	\N	\N	\N	\N	Папахи на башнях	rus		\N	\N	\N	\N	\N	\N	imported	t	good	f	0	2026-07-10 17:16:17.082963	2026-07-10 17:16:17.082963	2026-07-10 17:16:17.082963	\N	папахи на башнях	\N	\N
156	153	\N	\N	\N	\N	Подземное небо	rus		\N	\N	\N	\N	\N	\N	imported	t	good	f	0	2026-07-10 17:16:17.087774	2026-07-10 17:16:17.087774	2026-07-10 17:16:17.087774	\N	подземное небо	\N	\N
157	154	\N	\N	\N	\N	Последняя шутка воина	rus		\N	\N	\N	\N	\N	\N	imported	t	good	f	0	2026-07-10 17:16:17.092582	2026-07-10 17:16:17.092582	2026-07-10 17:16:17.092582	\N	последняя шутка воина	\N	\N
158	155	\N	\N	\N	\N	Принц Госплана	rus		\N	\N	\N	\N	\N	\N	imported	t	good	f	0	2026-07-10 17:16:17.104327	2026-07-10 17:16:17.104327	2026-07-10 17:16:17.104327	\N	принц госплана	\N	\N
159	156	\N	\N	\N	\N	Проблема верволка в средней полосе	rus		\N	\N	\N	\N	\N	\N	imported	t	good	f	0	2026-07-10 17:16:17.114427	2026-07-10 17:16:17.114427	2026-07-10 17:16:17.114427	\N	проблема верволка в средней полосе	\N	\N
160	157	\N	\N	\N	\N	Происхождение видов	rus		\N	\N	\N	\N	\N	\N	imported	t	good	f	0	2026-07-10 17:16:17.119512	2026-07-10 17:16:17.119512	2026-07-10 17:16:17.119512	\N	происхождение видов	\N	\N
161	158	978-5-699-30532-2	\N	\N	\N	Пространство Фридмана	rus		2008	\N	\N	\N	\N	\N	imported	t	good	f	0	2026-07-10 17:16:17.126524	2026-07-10 17:16:17.126524	2026-07-10 17:16:17.126524	\N	пространство фридмана	\N	\N
162	159	\N	\N	\N	\N	П5: Прощальные песни политических пигмеев Пиндостана	rus		2008	\N	\N	\N	\N	\N	imported	t	good	f	0	2026-07-10 17:16:17.152326	2026-07-10 17:16:17.152326	2026-07-10 17:16:17.152326	\N	п5: прощальные песни политических пигмеев пиндостана	\N	\N
163	160	\N	\N	\N	\N	Реконструктор	rus		\N	\N	\N	\N	\N	\N	imported	t	good	f	0	2026-07-10 17:16:17.157249	2026-07-10 17:16:17.157249	2026-07-10 17:16:17.157249	\N	реконструктор	\N	\N
164	161	\N	\N	\N	\N	СССР Тайшоу Чжуань	rus		\N	\N	\N	\N	\N	\N	imported	t	good	f	0	2026-07-10 17:16:17.162833	2026-07-10 17:16:17.162833	2026-07-10 17:16:17.162833	\N	ссср тайшоу чжуань	\N	\N
165	162	\N	\N	\N	\N	Свет горизонта	rus		\N	\N	\N	\N	\N	\N	imported	t	good	f	0	2026-07-10 17:16:17.169729	2026-07-10 17:16:17.169729	2026-07-10 17:16:17.169729	\N	свет горизонта	\N	\N
166	163	\N	\N	\N	\N	Святочный киберпанк 117.dir	rus		\N	\N	\N	\N	\N	\N	imported	t	good	f	0	2026-07-10 17:16:17.175617	2026-07-10 17:16:17.175617	2026-07-10 17:16:17.175617	\N	святочный киберпанк 117.dir	\N	\N
167	164	5-699-08445-2	\N	\N	\N	Священная книга оборотня	rus		2004	\N	\N	\N	\N	\N	imported	t	good	f	0	2026-07-10 17:16:17.217133	2026-07-10 17:16:17.217133	2026-07-10 17:16:17.217133	\N	священная книга оборотня	\N	\N
168	165	\N	\N	\N	\N	Синий фонарь	rus		\N	\N	\N	\N	\N	\N	imported	t	good	f	0	2026-07-10 17:16:17.223892	2026-07-10 17:16:17.223892	2026-07-10 17:16:17.223892	\N	синий фонарь	\N	\N
169	165	5-85950-013-0	\N	\N	\N	Синий фонарь	rus		\N	\N	\N	\N	\N	\N	imported	t	good	f	0	2026-07-10 17:16:17.274981	2026-07-10 17:16:17.274981	2026-07-10 17:16:17.274981	\N	синий фонарь	\N	\N
170	166	\N	\N	\N	\N	Спи	rus		\N	\N	\N	\N	\N	\N	imported	t	good	f	0	2026-07-10 17:16:17.282202	2026-07-10 17:16:17.282202	2026-07-10 17:16:17.282202	\N	спи	\N	\N
171	167	\N	\N	\N	\N	Тайм-аут, или Вечерняя Москва	rus		\N	\N	\N	\N	\N	\N	imported	t	good	f	0	2026-07-10 17:16:17.287425	2026-07-10 17:16:17.287425	2026-07-10 17:16:17.287425	\N	тайм-аут, или вечерняя москва	\N	\N
172	168	\N	\N	\N	\N	Тарзанка	rus		\N	\N	\N	\N	\N	\N	imported	t	good	f	0	2026-07-10 17:16:17.292974	2026-07-10 17:16:17.292974	2026-07-10 17:16:17.292974	\N	тарзанка	\N	\N
173	169	\N	\N	\N	\N	Тхаги	rus		\N	\N	\N	\N	\N	\N	imported	t	good	f	0	2026-07-10 17:16:17.301536	2026-07-10 17:16:17.301536	2026-07-10 17:16:17.301536	\N	тхаги	\N	\N
174	170	\N	\N	\N	\N	Ухряб	rus		\N	\N	\N	\N	\N	\N	imported	t	good	f	0	2026-07-10 17:16:17.307023	2026-07-10 17:16:17.307023	2026-07-10 17:16:17.307023	\N	ухряб	\N	\N
175	171	\N	\N	\N	\N	Фокус-группа (Сборник)	rus		\N	\N	\N	\N	\N	\N	imported	t	good	f	0	2026-07-10 17:16:17.337222	2026-07-10 17:16:17.337222	2026-07-10 17:16:17.337222	\N	фокус-группа (сборник)	\N	\N
176	172	\N	\N	\N	\N	Хрустальный мир	rus		\N	\N	\N	\N	\N	\N	imported	t	good	f	0	2026-07-10 17:16:17.345142	2026-07-10 17:16:17.345142	2026-07-10 17:16:17.345142	\N	хрустальный мир	\N	\N
177	173	5-9560-0083-Х	\N	\N	\N	Чапаев и Пустота	rus		1996	\N	\N	\N	\N	\N	imported	t	good	f	0	2026-07-10 17:16:17.38856	2026-07-10 17:16:17.38856	2026-07-10 17:16:17.38856	\N	чапаев и пустота	\N	\N
178	174	5-699-15621-6	\N	\N	\N	Числа	rus		2005	\N	\N	\N	\N	\N	imported	t	good	f	0	2026-07-10 17:16:17.406061	2026-07-10 17:16:17.406061	2026-07-10 17:16:17.406061	\N	числа	\N	\N
179	175	\N	\N	\N	\N	Шлем ужаса	rus		2005	\N	\N	\N	\N	\N	imported	t	good	f	0	2026-07-10 17:16:17.42286	2026-07-10 17:16:17.42286	2026-07-10 17:16:17.42286	\N	шлем ужаса	\N	\N
180	176	\N	\N	\N	\N	Эссе, статьи	rus		\N	\N	\N	\N	\N	\N	imported	t	good	f	0	2026-07-10 17:16:17.443056	2026-07-10 17:16:17.443056	2026-07-10 17:16:17.443056	\N	эссе, статьи	\N	\N
181	177	978-5-699-83417-4	\N	\N	\N	Смотритель. Том 1. Орден желтого флага	rus		2015	\N	\N	\N	\N	\N	imported	t	good	f	0	2026-07-10 17:16:17.507905	2026-07-10 17:16:17.507905	2026-07-10 17:16:17.507905	\N	смотритель. том 1. орден желтого флага	\N	\N
182	178	978-5-699-83419-8	\N	\N	\N	Смотритель. Книга 2. Железная бездна	rus		2015	\N	\N	\N	\N	\N	imported	t	good	f	0	2026-07-10 17:16:17.566502	2026-07-10 17:16:17.566502	2026-07-10 17:16:17.566502	\N	смотритель. книга 2. железная бездна	\N	\N
183	179	\N	\N	\N	\N	Пелевин В. - Круть (Трансгуманизм - 4) - 2024.a4.pdf	eng		\N	\N	\N	\N	\N	\N	imported	t	good	f	0	2026-07-10 17:16:17.606896	2026-07-10 17:16:17.606896	2026-07-10 17:16:17.606896	\N	пелевин в. - круть (трансгуманизм - 4) - 2024.a4.pdf	\N	\N
184	180	\N	\N	\N	\N	Круть	rus		2024	\N	\N	\N	\N	\N	imported	t	good	f	0	2026-07-10 17:16:17.664933	2026-07-10 17:16:17.664933	2026-07-10 17:16:17.664933	\N	круть	\N	\N
185	181	5-17-026471-2	\N	\N	\N	Апология Сократа	rus		\N	\N	\N	\N	\N	\N	imported	t	good	f	0	2026-07-10 17:16:17.677118	2026-07-10 17:16:17.677118	2026-07-10 17:16:17.677118	\N	апология сократа	\N	\N
186	182	\N	\N	\N	\N	Диалоги	rus		\N	\N	\N	\N	\N	\N	imported	t	good	f	0	2026-07-10 17:16:18.012986	2026-07-10 17:16:18.012986	2026-07-10 17:16:18.012986	\N	диалоги	\N	\N
187	183	978-5-91657-304-6	\N	\N	\N	Пелевин и поколение пустоты	rus		2012	\N	\N	\N	\N	\N	imported	t	good	f	0	2026-07-10 17:16:18.055594	2026-07-10 17:16:18.055594	2026-07-10 17:16:18.055594	\N	пелевин и поколение пустоты	\N	\N
188	184	\N	\N	\N	\N	Психология влияния	eng		\N	\N	\N	\N	\N	\N	imported	t	good	f	0	2026-07-10 17:16:50.755504	2026-07-10 17:16:50.755504	2026-07-10 17:16:50.755504	\N	психология влияния	\N	\N
189	185	978-5-89678-204-9	\N	\N	\N	Свобода Шамана	rus		2010	\N	\N	\N	\N	\N	imported	t	good	f	0	2026-07-10 17:16:50.771508	2026-07-10 17:16:50.771508	2026-07-10 17:16:50.771508	\N	свобода шамана	\N	\N
190	186	5-91250-173-6	\N	\N	\N	Хохот шамана	rus		\N	\N	\N	\N	\N	\N	imported	t	good	f	0	2026-07-10 17:16:50.785944	2026-07-10 17:16:50.785944	2026-07-10 17:16:50.785944	\N	хохот шамана	\N	\N
191	187	\N	\N	\N	\N	Шаманский Лес	rus		\N	\N	\N	\N	\N	\N	imported	t	good	f	0	2026-07-10 17:16:50.803971	2026-07-10 17:16:50.803971	2026-07-10 17:16:50.803971	\N	шаманский лес	\N	\N
192	188	\N	\N	\N	\N	Будущая запрещенная книга	rus		\N	\N	\N	\N	\N	\N	imported	t	good	f	0	2026-07-10 17:16:50.80986	2026-07-10 17:16:50.80986	2026-07-10 17:16:50.80986	\N	будущая запрещенная книга	\N	\N
193	189	\N	\N	\N	\N	Виктор Пелевин: эволюция в постмодернизме	rus		\N	\N	\N	\N	\N	\N	imported	t	good	f	0	2026-07-10 17:16:50.819177	2026-07-10 17:16:50.819177	2026-07-10 17:16:50.819177	\N	виктор пелевин: эволюция в постмодернизме	\N	\N
194	190	\N	\N	\N	\N	Как управлять рабами	rus	Олимп-Бизнес	2016	\N	\N	\N	\N	\N	imported	t	good	f	0	2026-07-10 17:16:50.8385	2026-07-10 17:16:50.8385	2026-07-10 17:16:50.8385	\N	как управлять рабами	\N	\N
195	191	\N	\N	\N	\N	Чёрный лебедь. Под знаком непредсказуемости	eng		\N	\N	\N	\N	\N	\N	imported	t	good	f	0	2026-07-10 17:17:30.231262	2026-07-10 17:17:30.231262	2026-07-10 17:17:30.231262	\N	черный лебедь. под знаком непредсказуемости	\N	\N
196	192	\N	\N	\N	\N	Четвертая промышленная революция. (Top Business Awards)	eng		\N	\N	\N	\N	\N	\N	imported	t	good	f	0	2026-07-10 17:18:05.551176	2026-07-10 17:18:05.551176	2026-07-10 17:18:05.551176	\N	четвертая промышленная революция. (top business awards)	\N	\N
197	193	\N	\N	\N	\N	Дневник писателя	rus		\N	\N	\N	\N	\N	\N	imported	t	good	f	0	2026-07-10 17:18:05.645105	2026-07-10 17:18:05.645105	2026-07-10 17:18:05.645105	\N	дневник писателя	\N	\N
198	194	\N	\N	\N	\N	косметическая химия.pdf	eng		\N	\N	\N	\N	\N	\N	imported	t	good	f	0	2026-07-10 17:18:06.053345	2026-07-10 17:18:06.053345	2026-07-10 17:18:06.053345	\N	косметическая химия.pdf	\N	\N
199	195	\N	\N	\N	\N	Математическая статистика	eng		\N	\N	\N	\N	\N	\N	imported	t	good	f	0	2026-07-10 17:18:42.731667	2026-07-10 17:18:42.731667	2026-07-10 17:18:42.731667	\N	математическая статистика	\N	\N
868	860	\N	\N	\N	\N	Внутренний огонь. Книга семь: применение левого ядра в повседневной жизни	eng		\N	\N	\N	\N	\N	\N	imported	t	good	f	0	2026-07-21 16:06:12.236656	2026-07-21 16:06:12.236656	2026-07-21 16:06:12.236656	\N	внутренний огонь. книга семь: применение левого ядра в повседневной жизни	\N	1
200	196	\N	\N	\N	\N	Test Book Part 1	eng	Self-published	2023	Self-published	0	\N	\N	First test book created via API	manual	t	good	f	0	2026-07-12 12:18:02.496071	2026-07-12 12:18:02.496071	2026-07-12 12:18:02.496071	\N	test book part 1	\N	\N
201	197	\N	\N	\N	\N	Test Book Part 2	eng	Self-published	2024	Self-published	0	\N	\N	Second test book created via API	manual	t	good	f	0	2026-07-12 12:18:02.503149	2026-07-12 12:18:02.503149	2026-07-12 12:18:02.503149	\N	test book part 2	\N	\N
446	442	\N	\N	\N	\N	Book Two	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-15 15:44:42.230332	2026-07-15 15:44:42.230332	2026-07-15 15:44:42.230332	\N	book two	\N	\N
202	198	\N	\N	\N	\N	Updated Book Title	eng	Self-published	2022	Self-published	0	\N	\N	Updated description	manual	t	good	f	0	2026-07-12 12:18:02.515928	2026-07-12 12:18:02.522407	2026-07-12 12:18:02.515928	\N	updated book title	\N	\N
207	203	\N	\N	\N	\N	Book One	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-12 12:18:02.606438	2026-07-20 20:55:13.308321	2026-07-12 12:18:02.606438	\N	book one	\N	\N
205	201	\N	\N	\N	\N	Book With ISBN Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-12 12:18:02.573093	2026-07-12 12:18:02.573093	2026-07-12 12:18:02.573093	\N	book with isbn test	\N	\N
206	202	\N	\N	\N	\N	Book Without ISBN	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-12 12:18:02.591618	2026-07-12 12:18:02.591618	2026-07-12 12:18:02.591618	\N	book without isbn	\N	\N
209	205	\N	\N	\N	\N	New Edition Title	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-12 12:18:02.624208	2026-07-12 12:18:02.629984	2026-07-12 12:18:02.624208	\N	new edition title	\N	\N
210	206	9780000000210	\N	\N	\N	Test Empty Strings	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-12 12:18:02.644655	2026-07-12 12:18:02.650586	2026-07-12 12:18:02.644655	\N	test empty strings	\N	\N
211	207	97800000002111	\N	\N	\N	Corrupted ISBN Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-12 12:18:02.663862	2026-07-12 12:18:02.670408	2026-07-12 12:18:02.663862	\N	corrupted isbn test	\N	\N
212	208	\N	\N	\N	\N	Remove Authors Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-12 12:18:02.997813	2026-07-12 12:18:02.997813	2026-07-12 12:18:02.997813	\N	remove authors test	\N	\N
213	209	\N	\N	\N	\N	Remove Genres Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-12 12:18:03.014685	2026-07-12 12:18:03.014685	2026-07-12 12:18:03.014685	\N	remove genres test	\N	\N
214	210	\N	\N	\N	\N	Remove Tags Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-12 12:18:03.031074	2026-07-12 12:18:03.031074	2026-07-12 12:18:03.031074	\N	remove tags test	\N	\N
215	211	\N	\N	\N	\N	Nil Authors Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-12 12:18:03.047847	2026-07-12 12:18:03.047847	2026-07-12 12:18:03.047847	\N	nil authors test	\N	\N
217	213	\N	\N	\N	\N	Test Book Part 1	eng	Self-published	2023	Self-published	0	\N	\N	First test book created via API	manual	t	good	f	0	2026-07-12 12:18:32.256662	2026-07-12 12:18:32.256662	2026-07-12 12:18:32.256662	\N	test book part 1	\N	\N
218	214	\N	\N	\N	\N	Test Book Part 2	eng	Self-published	2024	Self-published	0	\N	\N	Second test book created via API	manual	t	good	f	0	2026-07-12 12:18:32.262755	2026-07-12 12:18:32.262755	2026-07-12 12:18:32.262755	\N	test book part 2	\N	\N
219	215	\N	\N	\N	\N	Updated Book Title	eng	Self-published	2022	Self-published	0	\N	\N	Updated description	manual	t	good	f	0	2026-07-12 12:18:32.272855	2026-07-12 12:18:32.278639	2026-07-12 12:18:32.272855	\N	updated book title	\N	\N
221	217	\N	\N	\N	\N	Original Title	eng	Self-published	2025	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-12 12:18:32.308558	2026-07-12 12:18:32.316961	2026-07-12 12:18:32.308558	\N	original title	\N	\N
222	218	\N	\N	\N	\N	Book With ISBN Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-12 12:18:32.33166	2026-07-12 12:18:32.33166	2026-07-12 12:18:32.33166	\N	book with isbn test	\N	\N
223	219	\N	\N	\N	\N	Book Without ISBN	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-12 12:18:32.352074	2026-07-12 12:18:32.352074	2026-07-12 12:18:32.352074	\N	book without isbn	\N	\N
225	221	\N	\N	\N	\N	Book Two	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-12 12:18:32.375111	2026-07-12 12:18:32.375111	2026-07-12 12:18:32.375111	\N	book two	\N	\N
226	222	\N	\N	\N	\N	New Edition Title	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-12 12:18:32.386993	2026-07-12 12:18:32.39342	2026-07-12 12:18:32.386993	\N	new edition title	\N	\N
227	223	9780000000227	\N	\N	\N	Test Empty Strings	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-12 12:18:32.404918	2026-07-12 12:18:32.412049	2026-07-12 12:18:32.404918	\N	test empty strings	\N	\N
228	224	97800000002281	\N	\N	\N	Corrupted ISBN Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-12 12:18:32.421379	2026-07-12 12:18:32.428017	2026-07-12 12:18:32.421379	\N	corrupted isbn test	\N	\N
229	225	\N	\N	\N	\N	Remove Authors Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-12 12:18:32.756463	2026-07-12 12:18:32.756463	2026-07-12 12:18:32.756463	\N	remove authors test	\N	\N
230	226	\N	\N	\N	\N	Remove Genres Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-12 12:18:32.775359	2026-07-12 12:18:32.775359	2026-07-12 12:18:32.775359	\N	remove genres test	\N	\N
231	227	\N	\N	\N	\N	Remove Tags Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-12 12:18:32.796392	2026-07-12 12:18:32.796392	2026-07-12 12:18:32.796392	\N	remove tags test	\N	\N
232	228	\N	\N	\N	\N	Nil Authors Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-12 12:18:32.811545	2026-07-12 12:18:32.811545	2026-07-12 12:18:32.811545	\N	nil authors test	\N	\N
208	204	\N	\N	\N	\N	Book Two	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-12 12:18:02.612695	2026-07-13 11:39:42.297456	2026-07-12 12:18:02.612695	\N	book two	\N	\N
869	861	\N	\N	\N	\N	СилаБезмолвия. Пролог	eng		\N	\N	\N	\N	\N	\N	imported	t	good	f	0	2026-07-21 16:06:45.91521	2026-07-21 16:06:45.91521	2026-07-21 16:06:45.91521	\N	силабезмолвия. пролог	\N	1
440	436	\N	\N	\N	\N	Updated Book Title	eng	Self-published	2022	Self-published	0	\N	\N	Updated description	manual	t	good	f	0	2026-07-15 15:44:42.137584	2026-07-15 15:44:42.143027	2026-07-15 15:44:42.137584	\N	updated book title	\N	\N
1410	1402	\N	\N	\N	\N	Original Title	eng	Self-published	2025	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-27 10:21:49.346495	2026-07-27 10:21:49.354944	2026-07-27 10:21:49.346495	\N	original title	\N	\N
234	230	\N	\N	\N	\N	Test Book Part 1	eng	Self-published	2023	Self-published	0	\N	\N	First test book created via API	manual	t	good	f	0	2026-07-12 20:10:38.902451	2026-07-12 20:10:38.902451	2026-07-12 20:10:38.902451	\N	test book part 1	\N	\N
235	231	\N	\N	\N	\N	Test Book Part 2	eng	Self-published	2024	Self-published	0	\N	\N	Second test book created via API	manual	t	good	f	0	2026-07-12 20:10:38.909258	2026-07-12 20:10:38.909258	2026-07-12 20:10:38.909258	\N	test book part 2	\N	\N
224	220	\N	\N	\N	\N	Book One	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	3	2026-07-12 12:18:32.369151	2026-07-22 09:25:45.958699	2026-07-12 12:18:32.369151	\N	book one	\N	\N
236	232	\N	\N	\N	\N	Updated Book Title	eng	Self-published	2022	Self-published	0	\N	\N	Updated description	manual	t	good	f	0	2026-07-12 20:10:38.919663	2026-07-12 20:10:38.927411	2026-07-12 20:10:38.919663	\N	updated book title	\N	\N
267	263	\N	\N	\N	\N	Add Author Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-15 08:44:21.40457	2026-07-23 13:19:33.316987	2026-07-15 08:44:21.40457	\N	add author test	\N	\N
101	98	\N	\N	\N	\N	Ultima Тулеев, или Дао выборов	rus		\N	\N	\N	\N	\N	\N	imported	t	good	f	0	2026-07-10 17:16:16.314597	2026-07-24 19:13:13.44264	2026-07-10 17:16:16.314597	\N	ultima тулеев, или дао выборов	\N	\N
238	234	\N	\N	\N	\N	Original Title	eng	Self-published	2025	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-12 20:10:38.958668	2026-07-12 20:10:38.967268	2026-07-12 20:10:38.958668	\N	original title	\N	\N
239	235	\N	\N	\N	\N	Book With ISBN Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-12 20:10:38.980271	2026-07-12 20:10:38.980271	2026-07-12 20:10:38.980271	\N	book with isbn test	\N	\N
240	236	\N	\N	\N	\N	Book Without ISBN	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-12 20:10:38.99781	2026-07-12 20:10:38.99781	2026-07-12 20:10:38.99781	\N	book without isbn	\N	\N
242	238	\N	\N	\N	\N	Book Two	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-12 20:10:39.021881	2026-07-12 20:10:39.021881	2026-07-12 20:10:39.021881	\N	book two	\N	\N
243	239	\N	\N	\N	\N	New Edition Title	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-12 20:10:39.034065	2026-07-12 20:10:39.040125	2026-07-12 20:10:39.034065	\N	new edition title	\N	\N
244	240	9780000000244	\N	\N	\N	Test Empty Strings	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-12 20:10:39.049522	2026-07-12 20:10:39.056144	2026-07-12 20:10:39.049522	\N	test empty strings	\N	\N
245	241	97800000002451	\N	\N	\N	Corrupted ISBN Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-12 20:10:39.064685	2026-07-12 20:10:39.07085	2026-07-12 20:10:39.064685	\N	corrupted isbn test	\N	\N
246	242	\N	\N	\N	\N	Remove Authors Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-12 20:10:39.366897	2026-07-12 20:10:39.366897	2026-07-12 20:10:39.366897	\N	remove authors test	\N	\N
247	243	\N	\N	\N	\N	Remove Genres Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-12 20:10:39.383509	2026-07-12 20:10:39.383509	2026-07-12 20:10:39.383509	\N	remove genres test	\N	\N
248	244	\N	\N	\N	\N	Remove Tags Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-12 20:10:39.399412	2026-07-12 20:10:39.399412	2026-07-12 20:10:39.399412	\N	remove tags test	\N	\N
249	245	\N	\N	\N	\N	Nil Authors Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-12 20:10:39.4153	2026-07-12 20:10:39.4153	2026-07-12 20:10:39.4153	\N	nil authors test	\N	\N
250	246	\N	\N	\N	\N	Add Author Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-12 20:10:39.429535	2026-07-12 20:10:39.429535	2026-07-12 20:10:39.429535	\N	add author test	\N	\N
251	247	\N	\N	\N	\N	Test Book Part 1	eng	Self-published	2023	Self-published	0	\N	\N	First test book created via API	manual	t	good	f	0	2026-07-15 08:44:20.822797	2026-07-15 08:44:20.822797	2026-07-15 08:44:20.822797	\N	test book part 1	\N	\N
252	248	\N	\N	\N	\N	Test Book Part 2	eng	Self-published	2024	Self-published	0	\N	\N	Second test book created via API	manual	t	good	f	0	2026-07-15 08:44:20.829151	2026-07-15 08:44:20.829151	2026-07-15 08:44:20.829151	\N	test book part 2	\N	\N
253	249	\N	\N	\N	\N	Updated Book Title	eng	Self-published	2022	Self-published	0	\N	\N	Updated description	manual	t	good	f	0	2026-07-15 08:44:20.850686	2026-07-15 08:44:20.856851	2026-07-15 08:44:20.850686	\N	updated book title	\N	\N
255	251	\N	\N	\N	\N	Original Title	eng	Self-published	2025	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-15 08:44:20.894197	2026-07-15 08:44:20.902586	2026-07-15 08:44:20.894197	\N	original title	\N	\N
256	252	\N	\N	\N	\N	Book With ISBN Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-15 08:44:20.914861	2026-07-15 08:44:20.914861	2026-07-15 08:44:20.914861	\N	book with isbn test	\N	\N
257	253	\N	\N	\N	\N	Book Without ISBN	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-15 08:44:20.932181	2026-07-15 08:44:20.932181	2026-07-15 08:44:20.932181	\N	book without isbn	\N	\N
259	255	\N	\N	\N	\N	Book Two	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-15 08:44:20.953044	2026-07-15 08:44:20.953044	2026-07-15 08:44:20.953044	\N	book two	\N	\N
241	237	\N	\N	\N	\N	Book One	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-12 20:10:39.015863	2026-07-15 08:44:20.955277	2026-07-12 20:10:39.015863	\N	book one	\N	\N
260	256	\N	\N	\N	\N	New Edition Title	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-15 08:44:20.963106	2026-07-15 08:44:20.969253	2026-07-15 08:44:20.963106	\N	new edition title	\N	\N
261	257	9780000000261	\N	\N	\N	Test Empty Strings	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-15 08:44:20.979927	2026-07-15 08:44:20.987082	2026-07-15 08:44:20.979927	\N	test empty strings	\N	\N
262	258	97800000002621	\N	\N	\N	Corrupted ISBN Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-15 08:44:20.996975	2026-07-15 08:44:21.005241	2026-07-15 08:44:20.996975	\N	corrupted isbn test	\N	\N
263	259	\N	\N	\N	\N	Remove Authors Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-15 08:44:21.341228	2026-07-15 08:44:21.341228	2026-07-15 08:44:21.341228	\N	remove authors test	\N	\N
264	260	\N	\N	\N	\N	Remove Genres Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-15 08:44:21.357192	2026-07-15 08:44:21.357192	2026-07-15 08:44:21.357192	\N	remove genres test	\N	\N
265	261	\N	\N	\N	\N	Remove Tags Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-15 08:44:21.373614	2026-07-15 08:44:21.373614	2026-07-15 08:44:21.373614	\N	remove tags test	\N	\N
266	262	\N	\N	\N	\N	Nil Authors Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-15 08:44:21.390009	2026-07-15 08:44:21.390009	2026-07-15 08:44:21.390009	\N	nil authors test	\N	\N
268	264	\N	\N	\N	\N	Test Book Part 1	eng	Self-published	2023	Self-published	0	\N	\N	First test book created via API	manual	t	good	f	0	2026-07-15 11:10:54.455944	2026-07-15 11:10:54.455944	2026-07-15 11:10:54.455944	\N	test book part 1	\N	\N
269	265	\N	\N	\N	\N	Test Book Part 2	eng	Self-published	2024	Self-published	0	\N	\N	Second test book created via API	manual	t	good	f	0	2026-07-15 11:10:54.462577	2026-07-15 11:10:54.462577	2026-07-15 11:10:54.462577	\N	test book part 2	\N	\N
270	266	\N	\N	\N	\N	Updated Book Title	eng	Self-published	2022	Self-published	0	\N	\N	Updated description	manual	t	good	f	0	2026-07-15 11:10:54.473337	2026-07-15 11:10:54.479369	2026-07-15 11:10:54.473337	\N	updated book title	\N	\N
272	268	\N	\N	\N	\N	Original Title	eng	Self-published	2025	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-15 11:10:54.502927	2026-07-15 11:10:54.511345	2026-07-15 11:10:54.502927	\N	original title	\N	\N
273	269	\N	\N	\N	\N	Book With ISBN Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-15 11:10:54.522383	2026-07-15 11:10:54.522383	2026-07-15 11:10:54.522383	\N	book with isbn test	\N	\N
274	270	\N	\N	\N	\N	Book Without ISBN	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-15 11:10:54.53754	2026-07-15 11:10:54.53754	2026-07-15 11:10:54.53754	\N	book without isbn	\N	\N
276	272	\N	\N	\N	\N	Book Two	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-15 11:10:54.558517	2026-07-15 11:10:54.558517	2026-07-15 11:10:54.558517	\N	book two	\N	\N
275	271	\N	\N	\N	\N	Book One	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-15 11:10:54.552356	2026-07-15 11:17:16.169863	2026-07-15 11:10:54.552356	\N	book one	\N	\N
258	254	\N	\N	\N	\N	Book One	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-15 08:44:20.947464	2026-07-15 11:10:54.560804	2026-07-15 08:44:20.947464	\N	book one	\N	\N
886	878	\N	\N	\N	\N	Book Two	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-21 16:09:01.101245	2026-07-21 16:09:01.101245	2026-07-21 16:09:01.101245	\N	book two	\N	\N
277	273	\N	\N	\N	\N	New Edition Title	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-15 11:10:54.571237	2026-07-15 11:10:54.577975	2026-07-15 11:10:54.571237	\N	new edition title	\N	\N
278	274	9780000000278	\N	\N	\N	Test Empty Strings	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-15 11:10:54.58866	2026-07-15 11:10:54.594908	2026-07-15 11:10:54.58866	\N	test empty strings	\N	\N
445	441	\N	\N	\N	\N	Book One	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-15 15:44:42.224617	2026-07-16 00:00:18.438744	2026-07-15 15:44:42.224617	\N	book one	\N	\N
279	275	97800000002791	\N	\N	\N	Corrupted ISBN Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-15 11:10:54.604642	2026-07-15 11:10:54.611498	2026-07-15 11:10:54.604642	\N	corrupted isbn test	\N	\N
280	276	\N	\N	\N	\N	Remove Authors Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-15 11:10:54.892667	2026-07-15 11:10:54.892667	2026-07-15 11:10:54.892667	\N	remove authors test	\N	\N
281	277	\N	\N	\N	\N	Remove Genres Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-15 11:10:54.907261	2026-07-15 11:10:54.907261	2026-07-15 11:10:54.907261	\N	remove genres test	\N	\N
282	278	\N	\N	\N	\N	Remove Tags Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-15 11:10:54.925318	2026-07-15 11:10:54.925318	2026-07-15 11:10:54.925318	\N	remove tags test	\N	\N
283	279	\N	\N	\N	\N	Nil Authors Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-15 11:10:54.939409	2026-07-15 11:10:54.939409	2026-07-15 11:10:54.939409	\N	nil authors test	\N	\N
284	280	\N	\N	\N	\N	Add Author Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-15 11:10:54.953817	2026-07-15 11:10:54.953817	2026-07-15 11:10:54.953817	\N	add author test	\N	\N
285	281	\N	\N	\N	\N	Test Book Part 1	eng	Self-published	2023	Self-published	0	\N	\N	First test book created via API	manual	t	good	f	0	2026-07-15 11:17:16.059925	2026-07-15 11:17:16.059925	2026-07-15 11:17:16.059925	\N	test book part 1	\N	\N
286	282	\N	\N	\N	\N	Test Book Part 2	eng	Self-published	2024	Self-published	0	\N	\N	Second test book created via API	manual	t	good	f	0	2026-07-15 11:17:16.06657	2026-07-15 11:17:16.06657	2026-07-15 11:17:16.06657	\N	test book part 2	\N	\N
287	283	\N	\N	\N	\N	Updated Book Title	eng	Self-published	2022	Self-published	0	\N	\N	Updated description	manual	t	good	f	0	2026-07-15 11:17:16.075633	2026-07-15 11:17:16.081617	2026-07-15 11:17:16.075633	\N	updated book title	\N	\N
289	285	\N	\N	\N	\N	Original Title	eng	Self-published	2025	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-15 11:17:16.107962	2026-07-15 11:17:16.116379	2026-07-15 11:17:16.107962	\N	original title	\N	\N
290	286	\N	\N	\N	\N	Book With ISBN Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-15 11:17:16.13052	2026-07-15 11:17:16.13052	2026-07-15 11:17:16.13052	\N	book with isbn test	\N	\N
291	287	\N	\N	\N	\N	Book Without ISBN	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-15 11:17:16.14727	2026-07-15 11:17:16.14727	2026-07-15 11:17:16.14727	\N	book without isbn	\N	\N
293	289	\N	\N	\N	\N	Book Two	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-15 11:17:16.167357	2026-07-15 11:17:16.167357	2026-07-15 11:17:16.167357	\N	book two	\N	\N
294	290	\N	\N	\N	\N	New Edition Title	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-15 11:17:16.18005	2026-07-15 11:17:16.186057	2026-07-15 11:17:16.18005	\N	new edition title	\N	\N
295	291	9780000000295	\N	\N	\N	Test Empty Strings	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-15 11:17:16.195037	2026-07-15 11:17:16.20099	2026-07-15 11:17:16.195037	\N	test empty strings	\N	\N
296	292	97800000002961	\N	\N	\N	Corrupted ISBN Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-15 11:17:16.209241	2026-07-15 11:17:16.215359	2026-07-15 11:17:16.209241	\N	corrupted isbn test	\N	\N
297	293	\N	\N	\N	\N	Remove Authors Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-15 11:17:16.483495	2026-07-15 11:17:16.483495	2026-07-15 11:17:16.483495	\N	remove authors test	\N	\N
298	294	\N	\N	\N	\N	Remove Genres Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-15 11:17:16.498616	2026-07-15 11:17:16.498616	2026-07-15 11:17:16.498616	\N	remove genres test	\N	\N
299	295	\N	\N	\N	\N	Remove Tags Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-15 11:17:16.51308	2026-07-15 11:17:16.51308	2026-07-15 11:17:16.51308	\N	remove tags test	\N	\N
300	296	\N	\N	\N	\N	Nil Authors Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-15 11:17:16.526649	2026-07-15 11:17:16.526649	2026-07-15 11:17:16.526649	\N	nil authors test	\N	\N
301	297	\N	\N	\N	\N	Add Author Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-15 11:17:16.543109	2026-07-15 11:17:16.543109	2026-07-15 11:17:16.543109	\N	add author test	\N	\N
302	298	\N	\N	\N	\N	Test Book Part 1	eng	Self-published	2023	Self-published	0	\N	\N	First test book created via API	manual	t	good	f	0	2026-07-15 11:25:56.943335	2026-07-15 11:25:56.943335	2026-07-15 11:25:56.943335	\N	test book part 1	\N	\N
303	299	\N	\N	\N	\N	Test Book Part 2	eng	Self-published	2024	Self-published	0	\N	\N	Second test book created via API	manual	t	good	f	0	2026-07-15 11:25:56.949832	2026-07-15 11:25:56.949832	2026-07-15 11:25:56.949832	\N	test book part 2	\N	\N
304	300	\N	\N	\N	\N	Updated Book Title	eng	Self-published	2022	Self-published	0	\N	\N	Updated description	manual	t	good	f	0	2026-07-15 11:25:56.960728	2026-07-15 11:25:56.967544	2026-07-15 11:25:56.960728	\N	updated book title	\N	\N
306	302	\N	\N	\N	\N	Original Title	eng	Self-published	2025	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-15 11:25:56.993289	2026-07-15 11:25:57.001317	2026-07-15 11:25:56.993289	\N	original title	\N	\N
307	303	\N	\N	\N	\N	Book With ISBN Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-15 11:25:57.013661	2026-07-15 11:25:57.013661	2026-07-15 11:25:57.013661	\N	book with isbn test	\N	\N
308	304	\N	\N	\N	\N	Book Without ISBN	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-15 11:25:57.03051	2026-07-15 11:25:57.03051	2026-07-15 11:25:57.03051	\N	book without isbn	\N	\N
310	306	\N	\N	\N	\N	Book Two	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-15 11:25:57.052333	2026-07-15 11:25:57.052333	2026-07-15 11:25:57.052333	\N	book two	\N	\N
292	288	\N	\N	\N	\N	Book One	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-15 11:17:16.161643	2026-07-15 11:25:57.055413	2026-07-15 11:17:16.161643	\N	book one	\N	\N
311	307	\N	\N	\N	\N	New Edition Title	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-15 11:25:57.062967	2026-07-15 11:25:57.069552	2026-07-15 11:25:57.062967	\N	new edition title	\N	\N
312	308	9780000000312	\N	\N	\N	Test Empty Strings	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-15 11:25:57.080505	2026-07-15 11:25:57.086846	2026-07-15 11:25:57.080505	\N	test empty strings	\N	\N
313	309	97800000003131	\N	\N	\N	Corrupted ISBN Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-15 11:25:57.095274	2026-07-15 11:25:57.101804	2026-07-15 11:25:57.095274	\N	corrupted isbn test	\N	\N
314	310	\N	\N	\N	\N	Remove Authors Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-15 11:26:02.404508	2026-07-15 11:26:02.404508	2026-07-15 11:26:02.404508	\N	remove authors test	\N	\N
315	311	\N	\N	\N	\N	Remove Genres Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-15 11:26:02.419639	2026-07-15 11:26:02.419639	2026-07-15 11:26:02.419639	\N	remove genres test	\N	\N
316	312	\N	\N	\N	\N	Remove Tags Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-15 11:26:02.435509	2026-07-15 11:26:02.435509	2026-07-15 11:26:02.435509	\N	remove tags test	\N	\N
317	313	\N	\N	\N	\N	Nil Authors Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-15 11:26:02.451861	2026-07-15 11:26:02.451861	2026-07-15 11:26:02.451861	\N	nil authors test	\N	\N
319	315	\N	\N	\N	\N	Test Book Part 1	eng	Self-published	2023	Self-published	0	\N	\N	First test book created via API	manual	t	good	f	0	2026-07-15 12:31:47.060228	2026-07-15 12:31:47.060228	2026-07-15 12:31:47.060228	\N	test book part 1	\N	\N
320	316	\N	\N	\N	\N	Test Book Part 2	eng	Self-published	2024	Self-published	0	\N	\N	Second test book created via API	manual	t	good	f	0	2026-07-15 12:31:47.066953	2026-07-15 12:31:47.066953	2026-07-15 12:31:47.066953	\N	test book part 2	\N	\N
309	305	\N	\N	\N	\N	Book One	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-15 11:25:57.046539	2026-07-15 12:31:47.182945	2026-07-15 11:25:57.046539	\N	book one	\N	\N
321	317	\N	\N	\N	\N	Updated Book Title	eng	Self-published	2022	Self-published	0	\N	\N	Updated description	manual	t	good	f	0	2026-07-15 12:31:47.077078	2026-07-15 12:31:47.083869	2026-07-15 12:31:47.077078	\N	updated book title	\N	\N
1411	1403	\N	\N	\N	\N	Book With ISBN Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-27 10:21:49.368984	2026-07-27 10:21:49.368984	2026-07-27 10:21:49.368984	\N	book with isbn test	\N	\N
323	319	\N	\N	\N	\N	Original Title	eng	Self-published	2025	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-15 12:31:47.110753	2026-07-15 12:31:47.11878	2026-07-15 12:31:47.110753	\N	original title	\N	\N
324	320	\N	\N	\N	\N	Book With ISBN Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-15 12:31:47.135188	2026-07-15 12:31:47.135188	2026-07-15 12:31:47.135188	\N	book with isbn test	\N	\N
325	321	\N	\N	\N	\N	Book Without ISBN	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-15 12:31:47.150268	2026-07-15 12:31:47.150268	2026-07-15 12:31:47.150268	\N	book without isbn	\N	\N
327	323	\N	\N	\N	\N	Book Two	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-15 12:31:47.179814	2026-07-15 12:31:47.179814	2026-07-15 12:31:47.179814	\N	book two	\N	\N
328	324	\N	\N	\N	\N	New Edition Title	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-15 12:31:47.191611	2026-07-15 12:31:47.197846	2026-07-15 12:31:47.191611	\N	new edition title	\N	\N
329	325	9780000000329	\N	\N	\N	Test Empty Strings	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-15 12:31:47.207282	2026-07-15 12:31:47.213733	2026-07-15 12:31:47.207282	\N	test empty strings	\N	\N
330	326	97800000003301	\N	\N	\N	Corrupted ISBN Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-15 12:31:47.222	2026-07-15 12:31:47.228229	2026-07-15 12:31:47.222	\N	corrupted isbn test	\N	\N
331	327	\N	\N	\N	\N	Remove Authors Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-15 12:31:47.523246	2026-07-15 12:31:47.523246	2026-07-15 12:31:47.523246	\N	remove authors test	\N	\N
332	328	\N	\N	\N	\N	Remove Genres Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-15 12:31:47.541337	2026-07-15 12:31:47.541337	2026-07-15 12:31:47.541337	\N	remove genres test	\N	\N
333	329	\N	\N	\N	\N	Remove Tags Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-15 12:31:47.558714	2026-07-15 12:31:47.558714	2026-07-15 12:31:47.558714	\N	remove tags test	\N	\N
334	330	\N	\N	\N	\N	Nil Authors Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-15 12:31:47.575156	2026-07-15 12:31:47.575156	2026-07-15 12:31:47.575156	\N	nil authors test	\N	\N
335	331	\N	\N	\N	\N	Add Author Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-15 12:31:47.59113	2026-07-15 12:31:47.59113	2026-07-15 12:31:47.59113	\N	add author test	\N	\N
336	332	\N	\N	\N	\N	Test Book Part 1	eng	Self-published	2023	Self-published	0	\N	\N	First test book created via API	manual	t	good	f	0	2026-07-15 12:33:20.39546	2026-07-15 12:33:20.39546	2026-07-15 12:33:20.39546	\N	test book part 1	\N	\N
337	333	\N	\N	\N	\N	Test Book Part 2	eng	Self-published	2024	Self-published	0	\N	\N	Second test book created via API	manual	t	good	f	0	2026-07-15 12:33:20.40135	2026-07-15 12:33:20.40135	2026-07-15 12:33:20.40135	\N	test book part 2	\N	\N
338	334	\N	\N	\N	\N	Updated Book Title	eng	Self-published	2022	Self-published	0	\N	\N	Updated description	manual	t	good	f	0	2026-07-15 12:33:20.412097	2026-07-15 12:33:20.417823	2026-07-15 12:33:20.412097	\N	updated book title	\N	\N
340	336	\N	\N	\N	\N	Original Title	eng	Self-published	2025	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-15 12:33:20.444583	2026-07-15 12:33:20.453215	2026-07-15 12:33:20.444583	\N	original title	\N	\N
341	337	\N	\N	\N	\N	Book With ISBN Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-15 12:33:20.666521	2026-07-15 12:33:20.666521	2026-07-15 12:33:20.666521	\N	book with isbn test	\N	\N
342	338	\N	\N	\N	\N	Book Without ISBN	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-15 12:33:20.683895	2026-07-15 12:33:20.683895	2026-07-15 12:33:20.683895	\N	book without isbn	\N	\N
344	340	\N	\N	\N	\N	Book Two	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-15 12:33:20.707766	2026-07-15 12:33:20.707766	2026-07-15 12:33:20.707766	\N	book two	\N	\N
326	322	\N	\N	\N	\N	Book One	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-15 12:31:47.17389	2026-07-15 12:33:20.71019	2026-07-15 12:31:47.17389	\N	book one	\N	\N
345	341	\N	\N	\N	\N	New Edition Title	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-15 12:33:20.720266	2026-07-15 12:33:20.725933	2026-07-15 12:33:20.720266	\N	new edition title	\N	\N
346	342	9780000000346	\N	\N	\N	Test Empty Strings	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-15 12:33:20.737465	2026-07-15 12:33:20.743331	2026-07-15 12:33:20.737465	\N	test empty strings	\N	\N
347	343	97800000003471	\N	\N	\N	Corrupted ISBN Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-15 12:33:20.752633	2026-07-15 12:33:20.758772	2026-07-15 12:33:20.752633	\N	corrupted isbn test	\N	\N
348	344	\N	\N	\N	\N	Remove Authors Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-15 12:33:21.100618	2026-07-15 12:33:21.100618	2026-07-15 12:33:21.100618	\N	remove authors test	\N	\N
349	345	\N	\N	\N	\N	Remove Genres Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-15 12:33:21.11749	2026-07-15 12:33:21.11749	2026-07-15 12:33:21.11749	\N	remove genres test	\N	\N
350	346	\N	\N	\N	\N	Remove Tags Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-15 12:33:21.13349	2026-07-15 12:33:21.13349	2026-07-15 12:33:21.13349	\N	remove tags test	\N	\N
351	347	\N	\N	\N	\N	Nil Authors Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-15 12:33:21.147393	2026-07-15 12:33:21.147393	2026-07-15 12:33:21.147393	\N	nil authors test	\N	\N
353	349	\N	\N	\N	\N	Test Book Part 1	eng	Self-published	2023	Self-published	0	\N	\N	First test book created via API	manual	t	good	f	0	2026-07-15 12:38:23.826639	2026-07-15 12:38:23.826639	2026-07-15 12:38:23.826639	\N	test book part 1	\N	\N
354	350	\N	\N	\N	\N	Test Book Part 2	eng	Self-published	2024	Self-published	0	\N	\N	Second test book created via API	manual	t	good	f	0	2026-07-15 12:38:23.833383	2026-07-15 12:38:23.833383	2026-07-15 12:38:23.833383	\N	test book part 2	\N	\N
355	351	\N	\N	\N	\N	Updated Book Title	eng	Self-published	2022	Self-published	0	\N	\N	Updated description	manual	t	good	f	0	2026-07-15 12:38:23.845596	2026-07-15 12:38:23.851519	2026-07-15 12:38:23.845596	\N	updated book title	\N	\N
357	353	\N	\N	\N	\N	Original Title	eng	Self-published	2025	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-15 12:38:23.881484	2026-07-15 12:38:23.889412	2026-07-15 12:38:23.881484	\N	original title	\N	\N
358	354	\N	\N	\N	\N	Book With ISBN Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-15 12:38:23.905436	2026-07-15 12:38:23.905436	2026-07-15 12:38:23.905436	\N	book with isbn test	\N	\N
359	355	\N	\N	\N	\N	Book Without ISBN	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-15 12:38:23.921829	2026-07-15 12:38:23.921829	2026-07-15 12:38:23.921829	\N	book without isbn	\N	\N
361	357	\N	\N	\N	\N	Book Two	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-15 12:38:23.949812	2026-07-15 12:38:23.949812	2026-07-15 12:38:23.949812	\N	book two	\N	\N
343	339	\N	\N	\N	\N	Book One	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-15 12:33:20.701301	2026-07-15 12:38:23.952147	2026-07-15 12:33:20.701301	\N	book one	\N	\N
362	358	\N	\N	\N	\N	New Edition Title	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-15 12:38:23.961966	2026-07-15 12:38:23.968803	2026-07-15 12:38:23.961966	\N	new edition title	\N	\N
363	359	9780000000363	\N	\N	\N	Test Empty Strings	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-15 12:38:23.982726	2026-07-15 12:38:23.988433	2026-07-15 12:38:23.982726	\N	test empty strings	\N	\N
364	360	97800000003641	\N	\N	\N	Corrupted ISBN Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-15 12:38:23.9982	2026-07-15 12:38:24.004886	2026-07-15 12:38:23.9982	\N	corrupted isbn test	\N	\N
365	361	\N	\N	\N	\N	Remove Authors Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-15 12:38:24.325114	2026-07-15 12:38:24.325114	2026-07-15 12:38:24.325114	\N	remove authors test	\N	\N
366	362	\N	\N	\N	\N	Remove Genres Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-15 12:38:24.341511	2026-07-15 12:38:24.341511	2026-07-15 12:38:24.341511	\N	remove genres test	\N	\N
367	363	\N	\N	\N	\N	Remove Tags Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-15 12:38:24.378098	2026-07-15 12:38:24.378098	2026-07-15 12:38:24.378098	\N	remove tags test	\N	\N
360	356	\N	\N	\N	\N	Book One	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-15 12:38:23.942764	2026-07-15 13:22:41.155727	2026-07-15 12:38:23.942764	\N	book one	\N	\N
368	364	\N	\N	\N	\N	Nil Authors Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-15 12:38:24.395247	2026-07-15 12:38:24.395247	2026-07-15 12:38:24.395247	\N	nil authors test	\N	\N
370	366	\N	\N	\N	\N	Test Book Part 1	eng	Self-published	2023	Self-published	0	\N	\N	First test book created via API	manual	t	good	f	0	2026-07-15 13:22:41.042904	2026-07-15 13:22:41.042904	2026-07-15 13:22:41.042904	\N	test book part 1	\N	\N
371	367	\N	\N	\N	\N	Test Book Part 2	eng	Self-published	2024	Self-published	0	\N	\N	Second test book created via API	manual	t	good	f	0	2026-07-15 13:22:41.048772	2026-07-15 13:22:41.048772	2026-07-15 13:22:41.048772	\N	test book part 2	\N	\N
442	438	\N	\N	\N	\N	Original Title	eng	Self-published	2025	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-15 15:44:42.170426	2026-07-15 15:44:42.17851	2026-07-15 15:44:42.170426	\N	original title	\N	\N
372	368	\N	\N	\N	\N	Updated Book Title	eng	Self-published	2022	Self-published	0	\N	\N	Updated description	manual	t	good	f	0	2026-07-15 13:22:41.05998	2026-07-15 13:22:41.066432	2026-07-15 13:22:41.05998	\N	updated book title	\N	\N
1412	1404	\N	\N	\N	\N	Book Without ISBN	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-27 10:21:49.385398	2026-07-27 10:21:49.385398	2026-07-27 10:21:49.385398	\N	book without isbn	\N	\N
374	370	\N	\N	\N	\N	Original Title	eng	Self-published	2025	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-15 13:22:41.094024	2026-07-15 13:22:41.102321	2026-07-15 13:22:41.094024	\N	original title	\N	\N
375	371	\N	\N	\N	\N	Book With ISBN Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-15 13:22:41.115042	2026-07-15 13:22:41.115042	2026-07-15 13:22:41.115042	\N	book with isbn test	\N	\N
376	372	\N	\N	\N	\N	Book Without ISBN	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-15 13:22:41.131793	2026-07-15 13:22:41.131793	2026-07-15 13:22:41.131793	\N	book without isbn	\N	\N
369	365	\N	\N	\N	\N	Add Author Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	1	2026-07-15 12:38:24.409845	2026-07-25 15:42:15.81631	2026-07-15 12:38:24.409845	\N	add author test	\N	\N
378	374	\N	\N	\N	\N	Book Two	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-15 13:22:41.153154	2026-07-15 13:22:41.153154	2026-07-15 13:22:41.153154	\N	book two	\N	\N
379	375	\N	\N	\N	\N	New Edition Title	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-15 13:22:41.164743	2026-07-15 13:22:41.170546	2026-07-15 13:22:41.164743	\N	new edition title	\N	\N
380	376	9780000000380	\N	\N	\N	Test Empty Strings	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-15 13:22:41.181428	2026-07-15 13:22:41.187241	2026-07-15 13:22:41.181428	\N	test empty strings	\N	\N
381	377	97800000003811	\N	\N	\N	Corrupted ISBN Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-15 13:22:41.202014	2026-07-15 13:22:41.208545	2026-07-15 13:22:41.202014	\N	corrupted isbn test	\N	\N
382	378	\N	\N	\N	\N	Remove Authors Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-15 13:22:41.525658	2026-07-15 13:22:41.525658	2026-07-15 13:22:41.525658	\N	remove authors test	\N	\N
383	379	\N	\N	\N	\N	Remove Genres Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-15 13:22:41.542953	2026-07-15 13:22:41.542953	2026-07-15 13:22:41.542953	\N	remove genres test	\N	\N
384	380	\N	\N	\N	\N	Remove Tags Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-15 13:22:41.559167	2026-07-15 13:22:41.559167	2026-07-15 13:22:41.559167	\N	remove tags test	\N	\N
385	381	\N	\N	\N	\N	Nil Authors Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-15 13:22:41.574774	2026-07-15 13:22:41.574774	2026-07-15 13:22:41.574774	\N	nil authors test	\N	\N
387	383	\N	\N	\N	\N	Test Book Part 1	eng	Self-published	2023	Self-published	0	\N	\N	First test book created via API	manual	t	good	f	0	2026-07-15 14:50:32.622829	2026-07-15 14:50:32.622829	2026-07-15 14:50:32.622829	\N	test book part 1	\N	\N
388	384	\N	\N	\N	\N	Test Book Part 2	eng	Self-published	2024	Self-published	0	\N	\N	Second test book created via API	manual	t	good	f	0	2026-07-15 14:50:32.629095	2026-07-15 14:50:32.629095	2026-07-15 14:50:32.629095	\N	test book part 2	\N	\N
389	385	\N	\N	\N	\N	Updated Book Title	eng	Self-published	2022	Self-published	0	\N	\N	Updated description	manual	t	good	f	0	2026-07-15 14:50:32.639384	2026-07-15 14:50:32.645193	2026-07-15 14:50:32.639384	\N	updated book title	\N	\N
391	387	\N	\N	\N	\N	Original Title	eng	Self-published	2025	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-15 14:50:32.675058	2026-07-15 14:50:32.684519	2026-07-15 14:50:32.675058	\N	original title	\N	\N
392	388	\N	\N	\N	\N	Book With ISBN Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-15 14:50:32.697854	2026-07-15 14:50:32.697854	2026-07-15 14:50:32.697854	\N	book with isbn test	\N	\N
393	389	\N	\N	\N	\N	Book Without ISBN	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-15 14:50:32.716891	2026-07-15 14:50:32.716891	2026-07-15 14:50:32.716891	\N	book without isbn	\N	\N
395	391	\N	\N	\N	\N	Book Two	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-15 14:50:32.738559	2026-07-15 14:50:32.738559	2026-07-15 14:50:32.738559	\N	book two	\N	\N
377	373	\N	\N	\N	\N	Book One	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-15 13:22:41.147237	2026-07-15 14:50:32.740863	2026-07-15 13:22:41.147237	\N	book one	\N	\N
396	392	\N	\N	\N	\N	New Edition Title	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-15 14:50:32.751238	2026-07-15 14:50:32.756773	2026-07-15 14:50:32.751238	\N	new edition title	\N	\N
397	393	9780000000397	\N	\N	\N	Test Empty Strings	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-15 14:50:32.766676	2026-07-15 14:50:32.772328	2026-07-15 14:50:32.766676	\N	test empty strings	\N	\N
398	394	97800000003981	\N	\N	\N	Corrupted ISBN Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-15 14:50:32.782569	2026-07-15 14:50:32.788936	2026-07-15 14:50:32.782569	\N	corrupted isbn test	\N	\N
399	395	\N	\N	\N	\N	Remove Authors Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-15 14:50:33.095301	2026-07-15 14:50:33.095301	2026-07-15 14:50:33.095301	\N	remove authors test	\N	\N
400	396	\N	\N	\N	\N	Remove Genres Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-15 14:50:33.11146	2026-07-15 14:50:33.11146	2026-07-15 14:50:33.11146	\N	remove genres test	\N	\N
401	397	\N	\N	\N	\N	Remove Tags Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-15 14:50:33.12665	2026-07-15 14:50:33.12665	2026-07-15 14:50:33.12665	\N	remove tags test	\N	\N
402	398	\N	\N	\N	\N	Nil Authors Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-15 14:50:33.145271	2026-07-15 14:50:33.145271	2026-07-15 14:50:33.145271	\N	nil authors test	\N	\N
403	399	\N	\N	\N	\N	Add Author Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-15 14:50:33.159857	2026-07-15 14:50:33.159857	2026-07-15 14:50:33.159857	\N	add author test	\N	\N
404	400	\N	\N	\N	\N	Test Book Part 1	eng	Self-published	2023	Self-published	0	\N	\N	First test book created via API	manual	t	good	f	0	2026-07-15 14:57:25.212397	2026-07-15 14:57:25.212397	2026-07-15 14:57:25.212397	\N	test book part 1	\N	\N
405	401	\N	\N	\N	\N	Test Book Part 2	eng	Self-published	2024	Self-published	0	\N	\N	Second test book created via API	manual	t	good	f	0	2026-07-15 14:57:25.218584	2026-07-15 14:57:25.218584	2026-07-15 14:57:25.218584	\N	test book part 2	\N	\N
406	402	\N	\N	\N	\N	Updated Book Title	eng	Self-published	2022	Self-published	0	\N	\N	Updated description	manual	t	good	f	0	2026-07-15 14:57:25.230155	2026-07-15 14:57:25.235821	2026-07-15 14:57:25.230155	\N	updated book title	\N	\N
408	404	\N	\N	\N	\N	Original Title	eng	Self-published	2025	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-15 14:57:25.263708	2026-07-15 14:57:25.272737	2026-07-15 14:57:25.263708	\N	original title	\N	\N
409	405	\N	\N	\N	\N	Book With ISBN Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-15 14:57:25.287246	2026-07-15 14:57:25.287246	2026-07-15 14:57:25.287246	\N	book with isbn test	\N	\N
410	406	\N	\N	\N	\N	Book Without ISBN	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-15 14:57:25.305507	2026-07-15 14:57:25.305507	2026-07-15 14:57:25.305507	\N	book without isbn	\N	\N
412	408	\N	\N	\N	\N	Book Two	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-15 14:57:25.32718	2026-07-15 14:57:25.32718	2026-07-15 14:57:25.32718	\N	book two	\N	\N
394	390	\N	\N	\N	\N	Book One	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-15 14:50:32.733255	2026-07-15 14:57:25.329291	2026-07-15 14:50:32.733255	\N	book one	\N	\N
413	409	\N	\N	\N	\N	New Edition Title	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-15 14:57:25.340215	2026-07-15 14:57:25.346159	2026-07-15 14:57:25.340215	\N	new edition title	\N	\N
411	407	\N	\N	\N	\N	Book One	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-15 14:57:25.321305	2026-07-15 15:01:46.392099	2026-07-15 14:57:25.321305	\N	book one	\N	\N
414	410	9780000000414	\N	\N	\N	Test Empty Strings	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-15 14:57:25.356639	2026-07-15 14:57:25.362679	2026-07-15 14:57:25.356639	\N	test empty strings	\N	\N
443	439	\N	\N	\N	\N	Book With ISBN Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-15 15:44:42.194225	2026-07-15 15:44:42.194225	2026-07-15 15:44:42.194225	\N	book with isbn test	\N	\N
415	411	97800000004151	\N	\N	\N	Corrupted ISBN Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-15 14:57:25.372583	2026-07-15 14:57:25.378882	2026-07-15 14:57:25.372583	\N	corrupted isbn test	\N	\N
416	412	\N	\N	\N	\N	Remove Authors Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-15 14:57:25.692501	2026-07-15 14:57:25.692501	2026-07-15 14:57:25.692501	\N	remove authors test	\N	\N
417	413	\N	\N	\N	\N	Remove Genres Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-15 14:57:25.709233	2026-07-15 14:57:25.709233	2026-07-15 14:57:25.709233	\N	remove genres test	\N	\N
418	414	\N	\N	\N	\N	Remove Tags Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-15 14:57:25.725789	2026-07-15 14:57:25.725789	2026-07-15 14:57:25.725789	\N	remove tags test	\N	\N
419	415	\N	\N	\N	\N	Nil Authors Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-15 14:57:25.741801	2026-07-15 14:57:25.741801	2026-07-15 14:57:25.741801	\N	nil authors test	\N	\N
421	417	\N	\N	\N	\N	Test Book Part 1	eng	Self-published	2023	Self-published	0	\N	\N	First test book created via API	manual	t	good	f	0	2026-07-15 15:01:46.273532	2026-07-15 15:01:46.273532	2026-07-15 15:01:46.273532	\N	test book part 1	\N	\N
422	418	\N	\N	\N	\N	Test Book Part 2	eng	Self-published	2024	Self-published	0	\N	\N	Second test book created via API	manual	t	good	f	0	2026-07-15 15:01:46.279582	2026-07-15 15:01:46.279582	2026-07-15 15:01:46.279582	\N	test book part 2	\N	\N
428	424	\N	\N	\N	\N	Book One	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-15 15:01:46.38426	2026-07-15 15:44:42.232572	2026-07-15 15:01:46.38426	\N	book one	\N	\N
423	419	\N	\N	\N	\N	Updated Book Title	eng	Self-published	2022	Self-published	0	\N	\N	Updated description	manual	t	good	f	0	2026-07-15 15:01:46.290162	2026-07-15 15:01:46.296215	2026-07-15 15:01:46.290162	\N	updated book title	\N	\N
899	891	\N	\N	\N	\N	Original Title	eng	Self-published	2025	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-21 20:55:27.733729	2026-07-21 20:55:27.742464	2026-07-21 20:55:27.733729	\N	original title	\N	\N
447	443	\N	\N	\N	\N	New Edition Title	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-15 15:44:42.241793	2026-07-15 15:44:42.247344	2026-07-15 15:44:42.241793	\N	new edition title	\N	\N
425	421	\N	\N	\N	\N	Original Title	eng	Self-published	2025	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-15 15:01:46.326025	2026-07-15 15:01:46.334372	2026-07-15 15:01:46.326025	\N	original title	\N	\N
426	422	\N	\N	\N	\N	Book With ISBN Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-15 15:01:46.348715	2026-07-15 15:01:46.348715	2026-07-15 15:01:46.348715	\N	book with isbn test	\N	\N
427	423	\N	\N	\N	\N	Book Without ISBN	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-15 15:01:46.366337	2026-07-15 15:01:46.366337	2026-07-15 15:01:46.366337	\N	book without isbn	\N	\N
429	425	\N	\N	\N	\N	Book Two	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-15 15:01:46.389854	2026-07-15 15:01:46.389854	2026-07-15 15:01:46.389854	\N	book two	\N	\N
448	444	9780000000448	\N	\N	\N	Test Empty Strings	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-15 15:44:42.259819	2026-07-15 15:44:42.265733	2026-07-15 15:44:42.259819	\N	test empty strings	\N	\N
430	426	\N	\N	\N	\N	New Edition Title	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-15 15:01:46.412626	2026-07-15 15:01:46.418342	2026-07-15 15:01:46.412626	\N	new edition title	\N	\N
431	427	9780000000431	\N	\N	\N	Test Empty Strings	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-15 15:01:46.432084	2026-07-15 15:01:46.438519	2026-07-15 15:01:46.432084	\N	test empty strings	\N	\N
449	445	97800000004491	\N	\N	\N	Corrupted ISBN Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-15 15:44:42.275485	2026-07-15 15:44:42.281519	2026-07-15 15:44:42.275485	\N	corrupted isbn test	\N	\N
432	428	97800000004321	\N	\N	\N	Corrupted ISBN Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-15 15:01:46.450281	2026-07-15 15:01:46.456559	2026-07-15 15:01:46.450281	\N	corrupted isbn test	\N	\N
433	429	\N	\N	\N	\N	Remove Authors Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-15 15:01:46.882943	2026-07-15 15:01:46.882943	2026-07-15 15:01:46.882943	\N	remove authors test	\N	\N
434	430	\N	\N	\N	\N	Remove Genres Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-15 15:01:46.899738	2026-07-15 15:01:46.899738	2026-07-15 15:01:46.899738	\N	remove genres test	\N	\N
435	431	\N	\N	\N	\N	Remove Tags Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-15 15:01:46.9163	2026-07-15 15:01:46.9163	2026-07-15 15:01:46.9163	\N	remove tags test	\N	\N
436	432	\N	\N	\N	\N	Nil Authors Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-15 15:01:46.931549	2026-07-15 15:01:46.931549	2026-07-15 15:01:46.931549	\N	nil authors test	\N	\N
437	433	\N	\N	\N	\N	Add Author Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-15 15:01:46.946302	2026-07-15 15:01:46.946302	2026-07-15 15:01:46.946302	\N	add author test	\N	\N
450	446	\N	\N	\N	\N	Remove Authors Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-15 15:44:42.699648	2026-07-15 15:44:42.699648	2026-07-15 15:44:42.699648	\N	remove authors test	\N	\N
451	447	\N	\N	\N	\N	Remove Genres Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-15 15:44:42.715546	2026-07-15 15:44:42.715546	2026-07-15 15:44:42.715546	\N	remove genres test	\N	\N
452	448	\N	\N	\N	\N	Remove Tags Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-15 15:44:42.733712	2026-07-15 15:44:42.733712	2026-07-15 15:44:42.733712	\N	remove tags test	\N	\N
453	449	\N	\N	\N	\N	Nil Authors Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-15 15:44:42.749068	2026-07-15 15:44:42.749068	2026-07-15 15:44:42.749068	\N	nil authors test	\N	\N
454	450	\N	\N	\N	\N	Add Author Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-15 15:44:42.763748	2026-07-15 15:44:42.763748	2026-07-15 15:44:42.763748	\N	add author test	\N	\N
455	451	\N	\N	\N	\N	Test Book Part 1	eng	Self-published	2023	Self-published	0	\N	\N	First test book created via API	manual	t	good	f	0	2026-07-16 00:00:18.325954	2026-07-16 00:00:18.325954	2026-07-16 00:00:18.325954	\N	test book part 1	\N	\N
456	452	\N	\N	\N	\N	Test Book Part 2	eng	Self-published	2024	Self-published	0	\N	\N	Second test book created via API	manual	t	good	f	0	2026-07-16 00:00:18.331879	2026-07-16 00:00:18.331879	2026-07-16 00:00:18.331879	\N	test book part 2	\N	\N
457	453	\N	\N	\N	\N	Updated Book Title	eng	Self-published	2022	Self-published	0	\N	\N	Updated description	manual	t	good	f	0	2026-07-16 00:00:18.342737	2026-07-16 00:00:18.348497	2026-07-16 00:00:18.342737	\N	updated book title	\N	\N
459	455	\N	\N	\N	\N	Original Title	eng	Self-published	2025	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-16 00:00:18.375011	2026-07-16 00:00:18.382938	2026-07-16 00:00:18.375011	\N	original title	\N	\N
460	456	\N	\N	\N	\N	Book With ISBN Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-16 00:00:18.397376	2026-07-16 00:00:18.397376	2026-07-16 00:00:18.397376	\N	book with isbn test	\N	\N
461	457	\N	\N	\N	\N	Book Without ISBN	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-16 00:00:18.415814	2026-07-16 00:00:18.415814	2026-07-16 00:00:18.415814	\N	book without isbn	\N	\N
463	459	\N	\N	\N	\N	Book Two	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-16 00:00:18.436432	2026-07-16 00:00:18.436432	2026-07-16 00:00:18.436432	\N	book two	\N	\N
464	460	\N	\N	\N	\N	New Edition Title	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-16 00:00:18.452206	2026-07-16 00:00:18.458907	2026-07-16 00:00:18.452206	\N	new edition title	\N	\N
465	461	9780000000465	\N	\N	\N	Test Empty Strings	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-16 00:00:18.469925	2026-07-16 00:00:18.475666	2026-07-16 00:00:18.469925	\N	test empty strings	\N	\N
466	462	97800000004661	\N	\N	\N	Corrupted ISBN Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-16 00:00:18.486004	2026-07-16 00:00:18.493032	2026-07-16 00:00:18.486004	\N	corrupted isbn test	\N	\N
467	463	\N	\N	\N	\N	Remove Authors Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-16 00:00:18.91478	2026-07-16 00:00:18.91478	2026-07-16 00:00:18.91478	\N	remove authors test	\N	\N
471	467	\N	\N	\N	\N	Add Author Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-16 00:00:18.979901	2026-07-16 00:00:18.979901	2026-07-16 00:00:18.979901	\N	add author test	\N	\N
468	464	\N	\N	\N	\N	Remove Genres Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-16 00:00:18.931826	2026-07-16 00:00:18.931826	2026-07-16 00:00:18.931826	\N	remove genres test	\N	\N
469	465	\N	\N	\N	\N	Remove Tags Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-16 00:00:18.947752	2026-07-16 00:00:18.947752	2026-07-16 00:00:18.947752	\N	remove tags test	\N	\N
470	466	\N	\N	\N	\N	Nil Authors Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-16 00:00:18.962742	2026-07-16 00:00:18.962742	2026-07-16 00:00:18.962742	\N	nil authors test	\N	\N
472	468	\N	\N	\N	\N	Test Book Part 1	eng	Self-published	2023	Self-published	0	\N	\N	First test book created via API	manual	t	good	f	0	2026-07-16 05:13:10.193857	2026-07-16 05:13:10.193857	2026-07-16 05:13:10.193857	\N	test book part 1	\N	\N
473	469	\N	\N	\N	\N	Test Book Part 2	eng	Self-published	2024	Self-published	0	\N	\N	Second test book created via API	manual	t	good	f	0	2026-07-16 05:13:10.199859	2026-07-16 05:13:10.199859	2026-07-16 05:13:10.199859	\N	test book part 2	\N	\N
870	862	\N	\N	\N	\N	Активная сторона бесконечности. Том 10. Книги о мастерстве.	eng		\N	\N	\N	\N	\N	\N	imported	t	good	f	0	2026-07-21 16:07:37.630942	2026-07-21 16:07:37.630942	2026-07-21 16:07:37.630942	\N	активная сторона бесконечности. том 10. книги о мастерстве.	\N	1
474	470	\N	\N	\N	\N	Updated Book Title	eng	Self-published	2022	Self-published	0	\N	\N	Updated description	manual	t	good	f	0	2026-07-16 05:13:10.215997	2026-07-16 05:13:10.221953	2026-07-16 05:13:10.215997	\N	updated book title	\N	\N
871	863	\N	\N	\N	\N	Колесо времени	rus		1998	\N	\N	\N	\N	\N	imported	t	good	f	0	2026-07-21 16:07:37.647815	2026-07-21 16:07:37.647815	2026-07-21 16:07:37.647815	\N	колесо времени	\N	1
873	865	\N	\N	\N	\N	Empire V	rus		\N	\N	\N	\N	\N	\N	imported	t	good	f	0	2026-07-21 16:07:37.681712	2026-07-21 16:07:37.681712	2026-07-21 16:07:37.681712	\N	empire v	\N	1
874	866	978-5-699-21361-0	\N	\N	\N	Generation «П»	rus		1997	\N	\N	\N	\N	\N	imported	t	good	f	0	2026-07-21 16:07:37.72132	2026-07-21 16:07:37.72132	2026-07-21 16:07:37.72132	\N	generation «п»	\N	1
875	867	\N	\N	\N	\N	Relics. Раннее и неизданное (Сборник)	rus		\N	\N	\N	\N	\N	\N	imported	t	good	f	0	2026-07-21 16:07:37.758131	2026-07-21 16:07:37.758131	2026-07-21 16:07:37.758131	\N	relics. раннее и неизданное (сборник)	\N	1
876	868	978-5-699-53962-8	\N	\N	\N	S.N.U.F.F.	rus		2011	\N	\N	\N	\N	\N	imported	t	good	f	0	2026-07-21 16:07:37.817524	2026-07-21 16:07:37.817524	2026-07-21 16:07:37.817524	\N	s.n.u.f.f.	\N	1
877	869	978-5-699-23211-6	\N	\N	\N	Timeout, или Вечерняя Москва	rus		\N	\N	\N	\N	\N	\N	imported	t	good	f	0	2026-07-21 16:07:37.826585	2026-07-21 16:07:37.826585	2026-07-21 16:07:37.826585	\N	timeout, или вечерняя москва	\N	1
1414	1406	\N	\N	\N	\N	Book Two	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-27 10:21:49.408026	2026-07-27 10:21:49.408026	2026-07-27 10:21:49.408026	\N	book two	\N	\N
890	882	\N	\N	\N	\N	Remove Authors Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-21 16:09:01.657874	2026-07-21 16:09:01.657874	2026-07-21 16:09:01.657874	\N	remove authors test	\N	\N
893	885	\N	\N	\N	\N	Nil Authors Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-21 16:09:01.71826	2026-07-21 16:09:01.71826	2026-07-21 16:09:01.71826	\N	nil authors test	\N	\N
1413	1405	9780123456789	\N	\N	\N	Book One	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-27 10:21:49.402378	2026-07-27 10:21:49.410899	2026-07-27 10:21:49.402378	\N	book one	\N	\N
897	889	\N	\N	\N	\N	Updated Book Title	eng	Self-published	2022	Self-published	0	\N	\N	Updated description	manual	t	good	f	0	2026-07-21 20:55:27.698365	2026-07-21 20:55:27.704507	2026-07-21 20:55:27.698365	\N	updated book title	\N	\N
900	892	\N	\N	\N	\N	Book With ISBN Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-21 20:55:27.756819	2026-07-21 20:55:27.756819	2026-07-21 20:55:27.756819	\N	book with isbn test	\N	\N
903	895	\N	\N	\N	\N	Book Two	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-21 20:55:27.799325	2026-07-21 20:55:27.799325	2026-07-21 20:55:27.799325	\N	book two	\N	\N
885	877	\N	\N	\N	\N	Book One	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-21 16:09:01.094962	2026-07-21 20:55:27.802311	2026-07-21 16:09:01.094962	\N	book one	\N	\N
905	897	9780000000905	\N	\N	\N	Test Empty Strings	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-21 20:55:27.828743	2026-07-21 20:55:27.834974	2026-07-21 20:55:27.828743	\N	test empty strings	\N	\N
907	899	\N	\N	\N	\N	Remove Authors Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-21 20:55:28.306723	2026-07-21 20:55:28.306723	2026-07-21 20:55:28.306723	\N	remove authors test	\N	\N
909	901	\N	\N	\N	\N	Remove Tags Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-21 20:55:28.342165	2026-07-21 20:55:28.342165	2026-07-21 20:55:28.342165	\N	remove tags test	\N	\N
911	903	\N	\N	\N	\N	Add Author Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-21 20:55:28.374795	2026-07-21 20:55:28.374795	2026-07-21 20:55:28.374795	\N	add author test	\N	\N
914	906	\N	\N	\N	\N	Updated Book Title	eng	Self-published	2022	Self-published	0	\N	\N	Updated description	manual	t	good	f	0	2026-07-21 21:23:05.83424	2026-07-21 21:23:05.840803	2026-07-21 21:23:05.83424	\N	updated book title	\N	\N
916	908	\N	\N	\N	\N	Original Title	eng	Self-published	2025	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-21 21:23:05.872347	2026-07-21 21:23:05.881675	2026-07-21 21:23:05.872347	\N	original title	\N	\N
918	910	\N	\N	\N	\N	Book Without ISBN	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-21 21:23:05.91756	2026-07-21 21:23:05.91756	2026-07-21 21:23:05.91756	\N	book without isbn	\N	\N
902	894	\N	\N	\N	\N	Book One	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-21 20:55:27.792904	2026-07-21 21:23:05.944121	2026-07-21 20:55:27.792904	\N	book one	\N	\N
921	913	\N	\N	\N	\N	New Edition Title	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-21 21:23:05.955498	2026-07-21 21:23:05.96127	2026-07-21 21:23:05.955498	\N	new edition title	\N	\N
922	914	9780000000922	\N	\N	\N	Test Empty Strings	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-21 21:23:05.974623	2026-07-21 21:23:05.980788	2026-07-21 21:23:05.974623	\N	test empty strings	\N	\N
923	915	97800000009231	\N	\N	\N	Corrupted ISBN Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-21 21:23:05.991554	2026-07-21 21:23:05.998912	2026-07-21 21:23:05.991554	\N	corrupted isbn test	\N	\N
924	916	\N	\N	\N	\N	Remove Authors Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-21 21:23:06.482959	2026-07-21 21:23:06.482959	2026-07-21 21:23:06.482959	\N	remove authors test	\N	\N
925	917	\N	\N	\N	\N	Remove Genres Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-21 21:23:06.501982	2026-07-21 21:23:06.501982	2026-07-21 21:23:06.501982	\N	remove genres test	\N	\N
926	918	\N	\N	\N	\N	Remove Tags Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-21 21:23:06.520624	2026-07-21 21:23:06.520624	2026-07-21 21:23:06.520624	\N	remove tags test	\N	\N
927	919	\N	\N	\N	\N	Nil Authors Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-21 21:23:06.538302	2026-07-21 21:23:06.538302	2026-07-21 21:23:06.538302	\N	nil authors test	\N	\N
854	846	\N	\N	\N	\N	Жизнь реальна только тогда, когда "Я есть"	rus		\N	\N	\N	\N	\N	\N	imported	t	good	f	1	2026-07-21 16:02:00.936861	2026-07-24 18:59:12.835351	2026-07-21 16:02:00.936861	\N	жизнь реальна только тогда, когда "я есть"	\N	1
928	920	\N	\N	\N	\N	Add Author Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-21 21:23:06.552965	2026-07-22 09:29:02.893456	2026-07-21 21:23:06.552965	\N	add author test	\N	\N
204	200	\N	\N	\N	\N	Original Title	eng	Self-published	2025	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-12 12:18:02.548976	2026-07-22 09:29:12.958949	2026-07-12 12:18:02.548976	\N	original title	\N	\N
929	921	\N	\N	\N	\N	Test Book Part 1	eng	Self-published	2023	Self-published	0	\N	\N	First test book created via API	manual	t	good	f	0	2026-07-22 09:33:47.713172	2026-07-22 09:33:47.713172	2026-07-22 09:33:47.713172	\N	test book part 1	\N	\N
930	922	\N	\N	\N	\N	Test Book Part 2	eng	Self-published	2024	Self-published	0	\N	\N	Second test book created via API	manual	t	good	f	0	2026-07-22 09:33:47.719627	2026-07-22 09:33:47.719627	2026-07-22 09:33:47.719627	\N	test book part 2	\N	\N
919	911	\N	\N	\N	\N	Book One	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-21 21:23:05.93518	2026-07-22 09:33:47.849768	2026-07-21 21:23:05.93518	\N	book one	\N	\N
872	864	\N	\N	\N	\N	Виктор Пелевин и эффект Пустоты	rus		\N	\N	\N	\N	\N	\N	imported	t	good	f	0	2026-07-21 16:07:37.655156	2026-07-23 13:19:33.316987	2026-07-21 16:07:37.655156	\N	виктор пелевин и эффект пустоты	\N	1
476	472	\N	\N	\N	\N	Original Title	eng	Self-published	2025	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-16 05:13:10.27087	2026-07-16 05:13:10.279432	2026-07-16 05:13:10.27087	\N	original title	\N	\N
477	473	\N	\N	\N	\N	Book With ISBN Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-16 05:13:10.297237	2026-07-16 05:13:10.297237	2026-07-16 05:13:10.297237	\N	book with isbn test	\N	\N
478	474	\N	\N	\N	\N	Book Without ISBN	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-16 05:13:10.316674	2026-07-16 05:13:10.316674	2026-07-16 05:13:10.316674	\N	book without isbn	\N	\N
480	476	\N	\N	\N	\N	Book Two	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-16 05:13:10.346708	2026-07-16 05:13:10.346708	2026-07-16 05:13:10.346708	\N	book two	\N	\N
462	458	\N	\N	\N	\N	Book One	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-16 00:00:18.430578	2026-07-16 05:13:10.348944	2026-07-16 00:00:18.430578	\N	book one	\N	\N
481	477	\N	\N	\N	\N	New Edition Title	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-16 05:13:10.365052	2026-07-16 05:13:10.37054	2026-07-16 05:13:10.365052	\N	new edition title	\N	\N
482	478	9780000000482	\N	\N	\N	Test Empty Strings	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-16 05:13:10.385207	2026-07-16 05:13:10.391329	2026-07-16 05:13:10.385207	\N	test empty strings	\N	\N
483	479	97800000004831	\N	\N	\N	Corrupted ISBN Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-16 05:13:10.403912	2026-07-16 05:13:10.410307	2026-07-16 05:13:10.403912	\N	corrupted isbn test	\N	\N
484	480	\N	\N	\N	\N	Remove Authors Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-16 05:13:11.005959	2026-07-16 05:13:11.005959	2026-07-16 05:13:11.005959	\N	remove authors test	\N	\N
485	481	\N	\N	\N	\N	Remove Genres Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-16 05:13:11.024593	2026-07-16 05:13:11.024593	2026-07-16 05:13:11.024593	\N	remove genres test	\N	\N
486	482	\N	\N	\N	\N	Remove Tags Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-16 05:13:11.04492	2026-07-16 05:13:11.04492	2026-07-16 05:13:11.04492	\N	remove tags test	\N	\N
487	483	\N	\N	\N	\N	Nil Authors Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-16 05:13:11.063093	2026-07-16 05:13:11.063093	2026-07-16 05:13:11.063093	\N	nil authors test	\N	\N
488	484	\N	\N	\N	\N	Add Author Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-16 05:13:11.081085	2026-07-16 05:13:11.081085	2026-07-16 05:13:11.081085	\N	add author test	\N	\N
489	485	\N	\N	\N	\N	Test Book Part 1	eng	Self-published	2023	Self-published	0	\N	\N	First test book created via API	manual	t	good	f	0	2026-07-16 05:19:07.84064	2026-07-16 05:19:07.84064	2026-07-16 05:19:07.84064	\N	test book part 1	\N	\N
490	486	\N	\N	\N	\N	Test Book Part 2	eng	Self-published	2024	Self-published	0	\N	\N	Second test book created via API	manual	t	good	f	0	2026-07-16 05:19:07.847173	2026-07-16 05:19:07.847173	2026-07-16 05:19:07.847173	\N	test book part 2	\N	\N
491	487	\N	\N	\N	\N	Updated Book Title	eng	Self-published	2022	Self-published	0	\N	\N	Updated description	manual	t	good	f	0	2026-07-16 05:19:07.861986	2026-07-16 05:19:07.86806	2026-07-16 05:19:07.861986	\N	updated book title	\N	\N
493	489	\N	\N	\N	\N	Original Title	eng	Self-published	2025	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-16 05:19:07.901903	2026-07-16 05:19:07.910402	2026-07-16 05:19:07.901903	\N	original title	\N	\N
494	490	\N	\N	\N	\N	Book With ISBN Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-16 05:19:07.926913	2026-07-16 05:19:07.926913	2026-07-16 05:19:07.926913	\N	book with isbn test	\N	\N
495	491	\N	\N	\N	\N	Book Without ISBN	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-16 05:19:07.948587	2026-07-16 05:19:07.948587	2026-07-16 05:19:07.948587	\N	book without isbn	\N	\N
497	493	\N	\N	\N	\N	Book Two	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-16 05:19:07.973958	2026-07-16 05:19:07.973958	2026-07-16 05:19:07.973958	\N	book two	\N	\N
479	475	\N	\N	\N	\N	Book One	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-16 05:13:10.340774	2026-07-16 05:19:07.977256	2026-07-16 05:13:10.340774	\N	book one	\N	\N
498	494	\N	\N	\N	\N	New Edition Title	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-16 05:19:07.991001	2026-07-16 05:19:07.997164	2026-07-16 05:19:07.991001	\N	new edition title	\N	\N
499	495	9780000000499	\N	\N	\N	Test Empty Strings	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-16 05:19:08.012593	2026-07-16 05:19:08.018559	2026-07-16 05:19:08.012593	\N	test empty strings	\N	\N
500	496	97800000005001	\N	\N	\N	Corrupted ISBN Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-16 05:19:08.031522	2026-07-16 05:19:08.038156	2026-07-16 05:19:08.031522	\N	corrupted isbn test	\N	\N
501	497	\N	\N	\N	\N	Remove Authors Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-16 05:19:08.649389	2026-07-16 05:19:08.649389	2026-07-16 05:19:08.649389	\N	remove authors test	\N	\N
502	498	\N	\N	\N	\N	Remove Genres Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-16 05:19:08.668586	2026-07-16 05:19:08.668586	2026-07-16 05:19:08.668586	\N	remove genres test	\N	\N
503	499	\N	\N	\N	\N	Remove Tags Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-16 05:19:08.689174	2026-07-16 05:19:08.689174	2026-07-16 05:19:08.689174	\N	remove tags test	\N	\N
504	500	\N	\N	\N	\N	Nil Authors Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-16 05:19:08.709535	2026-07-16 05:19:08.709535	2026-07-16 05:19:08.709535	\N	nil authors test	\N	\N
505	501	\N	\N	\N	\N	Add Author Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-16 05:19:08.728588	2026-07-16 05:19:08.728588	2026-07-16 05:19:08.728588	\N	add author test	\N	\N
506	502	\N	\N	\N	\N	Test Book Part 1	eng	Self-published	2023	Self-published	0	\N	\N	First test book created via API	manual	t	good	f	0	2026-07-16 09:51:24.257907	2026-07-16 09:51:24.257907	2026-07-16 09:51:24.257907	\N	test book part 1	\N	\N
507	503	\N	\N	\N	\N	Test Book Part 2	eng	Self-published	2024	Self-published	0	\N	\N	Second test book created via API	manual	t	good	f	0	2026-07-16 09:51:24.263958	2026-07-16 09:51:24.263958	2026-07-16 09:51:24.263958	\N	test book part 2	\N	\N
508	504	\N	\N	\N	\N	Updated Book Title	eng	Self-published	2022	Self-published	0	\N	\N	Updated description	manual	t	good	f	0	2026-07-16 09:51:24.276236	2026-07-16 09:51:24.28262	2026-07-16 09:51:24.276236	\N	updated book title	\N	\N
510	506	\N	\N	\N	\N	Original Title	eng	Self-published	2025	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-16 09:51:24.312074	2026-07-16 09:51:24.319698	2026-07-16 09:51:24.312074	\N	original title	\N	\N
511	507	\N	\N	\N	\N	Book With ISBN Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-16 09:51:24.334036	2026-07-16 09:51:24.334036	2026-07-16 09:51:24.334036	\N	book with isbn test	\N	\N
512	508	\N	\N	\N	\N	Book Without ISBN	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-16 09:51:24.351483	2026-07-16 09:51:24.351483	2026-07-16 09:51:24.351483	\N	book without isbn	\N	\N
514	510	\N	\N	\N	\N	Book Two	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-16 09:51:24.373909	2026-07-16 09:51:24.373909	2026-07-16 09:51:24.373909	\N	book two	\N	\N
496	492	\N	\N	\N	\N	Book One	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-16 05:19:07.967495	2026-07-16 09:51:24.3762	2026-07-16 05:19:07.967495	\N	book one	\N	\N
515	511	\N	\N	\N	\N	New Edition Title	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-16 09:51:24.3866	2026-07-16 09:51:24.392529	2026-07-16 09:51:24.3866	\N	new edition title	\N	\N
516	512	9780000000516	\N	\N	\N	Test Empty Strings	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-16 09:51:24.403205	2026-07-16 09:51:24.409423	2026-07-16 09:51:24.403205	\N	test empty strings	\N	\N
517	513	97800000005171	\N	\N	\N	Corrupted ISBN Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-16 09:51:24.41911	2026-07-16 09:51:24.424901	2026-07-16 09:51:24.41911	\N	corrupted isbn test	\N	\N
518	514	\N	\N	\N	\N	Remove Authors Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-16 09:51:24.836834	2026-07-16 09:51:24.836834	2026-07-16 09:51:24.836834	\N	remove authors test	\N	\N
519	515	\N	\N	\N	\N	Remove Genres Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-16 09:51:24.853991	2026-07-16 09:51:24.853991	2026-07-16 09:51:24.853991	\N	remove genres test	\N	\N
520	516	\N	\N	\N	\N	Remove Tags Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-16 09:51:24.870654	2026-07-16 09:51:24.870654	2026-07-16 09:51:24.870654	\N	remove tags test	\N	\N
521	517	\N	\N	\N	\N	Nil Authors Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-16 09:51:24.885862	2026-07-16 09:51:24.885862	2026-07-16 09:51:24.885862	\N	nil authors test	\N	\N
522	518	\N	\N	\N	\N	Add Author Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-16 09:51:24.900963	2026-07-16 09:51:24.900963	2026-07-16 09:51:24.900963	\N	add author test	\N	\N
523	519	\N	\N	\N	\N	Test Book Part 1	eng	Self-published	2023	Self-published	0	\N	\N	First test book created via API	manual	t	good	f	0	2026-07-16 13:48:25.641523	2026-07-16 13:48:25.641523	2026-07-16 13:48:25.641523	\N	test book part 1	\N	\N
524	520	\N	\N	\N	\N	Test Book Part 2	eng	Self-published	2024	Self-published	0	\N	\N	Second test book created via API	manual	t	good	f	0	2026-07-16 13:48:25.647507	2026-07-16 13:48:25.647507	2026-07-16 13:48:25.647507	\N	test book part 2	\N	\N
878	870	\N	\N	\N	\N	Test Book Part 1	eng	Self-published	2023	Self-published	0	\N	\N	First test book created via API	manual	t	good	f	0	2026-07-21 16:09:00.978004	2026-07-21 16:09:00.978004	2026-07-21 16:09:00.978004	\N	test book part 1	\N	\N
525	521	\N	\N	\N	\N	Updated Book Title	eng	Self-published	2022	Self-published	0	\N	\N	Updated description	manual	t	good	f	0	2026-07-16 13:48:25.659262	2026-07-16 13:48:25.664817	2026-07-16 13:48:25.659262	\N	updated book title	\N	\N
894	886	\N	\N	\N	\N	Add Author Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-21 16:09:01.734127	2026-07-21 16:09:01.734127	2026-07-21 16:09:01.734127	\N	add author test	\N	\N
527	523	\N	\N	\N	\N	Original Title	eng	Self-published	2025	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-16 13:48:25.691298	2026-07-16 13:48:25.699891	2026-07-16 13:48:25.691298	\N	original title	\N	\N
528	524	\N	\N	\N	\N	Book With ISBN Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-16 13:48:25.713685	2026-07-16 13:48:25.713685	2026-07-16 13:48:25.713685	\N	book with isbn test	\N	\N
529	525	\N	\N	\N	\N	Book Without ISBN	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-16 13:48:25.732219	2026-07-16 13:48:25.732219	2026-07-16 13:48:25.732219	\N	book without isbn	\N	\N
531	527	\N	\N	\N	\N	Book Two	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-16 13:48:25.754315	2026-07-16 13:48:25.754315	2026-07-16 13:48:25.754315	\N	book two	\N	\N
513	509	\N	\N	\N	\N	Book One	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-16 09:51:24.367311	2026-07-16 13:48:25.756428	2026-07-16 09:51:24.367311	\N	book one	\N	\N
532	528	\N	\N	\N	\N	New Edition Title	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-16 13:48:25.765161	2026-07-16 13:48:25.770816	2026-07-16 13:48:25.765161	\N	new edition title	\N	\N
533	529	9780000000533	\N	\N	\N	Test Empty Strings	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-16 13:48:25.781191	2026-07-16 13:48:25.786836	2026-07-16 13:48:25.781191	\N	test empty strings	\N	\N
534	530	97800000005341	\N	\N	\N	Corrupted ISBN Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-16 13:48:25.796213	2026-07-16 13:48:25.802593	2026-07-16 13:48:25.796213	\N	corrupted isbn test	\N	\N
535	531	\N	\N	\N	\N	Remove Authors Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-16 13:48:26.214669	2026-07-16 13:48:26.214669	2026-07-16 13:48:26.214669	\N	remove authors test	\N	\N
536	532	\N	\N	\N	\N	Remove Genres Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-16 13:48:26.230894	2026-07-16 13:48:26.230894	2026-07-16 13:48:26.230894	\N	remove genres test	\N	\N
537	533	\N	\N	\N	\N	Remove Tags Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-16 13:48:26.246806	2026-07-16 13:48:26.246806	2026-07-16 13:48:26.246806	\N	remove tags test	\N	\N
538	534	\N	\N	\N	\N	Nil Authors Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-16 13:48:26.261889	2026-07-16 13:48:26.261889	2026-07-16 13:48:26.261889	\N	nil authors test	\N	\N
540	536	\N	\N	\N	\N	Test Book Part 1	eng	Self-published	2023	Self-published	0	\N	\N	First test book created via API	manual	t	good	f	0	2026-07-16 14:37:13.639082	2026-07-16 14:37:13.639082	2026-07-16 14:37:13.639082	\N	test book part 1	\N	\N
541	537	\N	\N	\N	\N	Test Book Part 2	eng	Self-published	2024	Self-published	0	\N	\N	Second test book created via API	manual	t	good	f	0	2026-07-16 14:37:13.645387	2026-07-16 14:37:13.645387	2026-07-16 14:37:13.645387	\N	test book part 2	\N	\N
542	538	\N	\N	\N	\N	Updated Book Title	eng	Self-published	2022	Self-published	0	\N	\N	Updated description	manual	t	good	f	0	2026-07-16 14:37:13.656564	2026-07-16 14:37:13.662324	2026-07-16 14:37:13.656564	\N	updated book title	\N	\N
544	540	\N	\N	\N	\N	Original Title	eng	Self-published	2025	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-16 14:37:13.688819	2026-07-16 14:37:13.698099	2026-07-16 14:37:13.688819	\N	original title	\N	\N
545	541	\N	\N	\N	\N	Book With ISBN Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-16 14:37:13.722114	2026-07-16 14:37:13.722114	2026-07-16 14:37:13.722114	\N	book with isbn test	\N	\N
546	542	\N	\N	\N	\N	Book Without ISBN	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-16 14:37:13.741516	2026-07-16 14:37:13.741516	2026-07-16 14:37:13.741516	\N	book without isbn	\N	\N
548	544	\N	\N	\N	\N	Book Two	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-16 14:37:13.763147	2026-07-16 14:37:13.763147	2026-07-16 14:37:13.763147	\N	book two	\N	\N
530	526	\N	\N	\N	\N	Book One	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-16 13:48:25.748534	2026-07-16 14:37:13.76543	2026-07-16 13:48:25.748534	\N	book one	\N	\N
549	545	\N	\N	\N	\N	New Edition Title	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-16 14:37:13.774287	2026-07-16 14:37:13.780103	2026-07-16 14:37:13.774287	\N	new edition title	\N	\N
550	546	9780000000550	\N	\N	\N	Test Empty Strings	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-16 14:37:13.790365	2026-07-16 14:37:13.796105	2026-07-16 14:37:13.790365	\N	test empty strings	\N	\N
551	547	97800000005511	\N	\N	\N	Corrupted ISBN Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-16 14:37:13.816803	2026-07-16 14:37:13.823269	2026-07-16 14:37:13.816803	\N	corrupted isbn test	\N	\N
552	548	\N	\N	\N	\N	Remove Authors Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-16 14:37:14.240742	2026-07-16 14:37:14.240742	2026-07-16 14:37:14.240742	\N	remove authors test	\N	\N
553	549	\N	\N	\N	\N	Remove Genres Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-16 14:37:14.257462	2026-07-16 14:37:14.257462	2026-07-16 14:37:14.257462	\N	remove genres test	\N	\N
554	550	\N	\N	\N	\N	Remove Tags Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-16 14:37:14.272864	2026-07-16 14:37:14.272864	2026-07-16 14:37:14.272864	\N	remove tags test	\N	\N
555	551	\N	\N	\N	\N	Nil Authors Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-16 14:37:14.290553	2026-07-16 14:37:14.290553	2026-07-16 14:37:14.290553	\N	nil authors test	\N	\N
557	553	\N	\N	\N	\N	Test Book Part 1	eng	Self-published	2023	Self-published	0	\N	\N	First test book created via API	manual	t	good	f	0	2026-07-16 14:42:15.225621	2026-07-16 14:42:15.225621	2026-07-16 14:42:15.225621	\N	test book part 1	\N	\N
558	554	\N	\N	\N	\N	Test Book Part 2	eng	Self-published	2024	Self-published	0	\N	\N	Second test book created via API	manual	t	good	f	0	2026-07-16 14:42:15.232175	2026-07-16 14:42:15.232175	2026-07-16 14:42:15.232175	\N	test book part 2	\N	\N
559	555	\N	\N	\N	\N	Updated Book Title	eng	Self-published	2022	Self-published	0	\N	\N	Updated description	manual	t	good	f	0	2026-07-16 14:42:15.245018	2026-07-16 14:42:15.25122	2026-07-16 14:42:15.245018	\N	updated book title	\N	\N
561	557	\N	\N	\N	\N	Original Title	eng	Self-published	2025	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-16 14:42:15.279652	2026-07-16 14:42:15.289989	2026-07-16 14:42:15.279652	\N	original title	\N	\N
562	558	\N	\N	\N	\N	Book With ISBN Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-16 14:42:15.305864	2026-07-16 14:42:15.305864	2026-07-16 14:42:15.305864	\N	book with isbn test	\N	\N
563	559	\N	\N	\N	\N	Book Without ISBN	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-16 14:42:15.324802	2026-07-16 14:42:15.324802	2026-07-16 14:42:15.324802	\N	book without isbn	\N	\N
565	561	\N	\N	\N	\N	Book Two	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-16 14:42:15.345205	2026-07-16 14:42:15.345205	2026-07-16 14:42:15.345205	\N	book two	\N	\N
547	543	\N	\N	\N	\N	Book One	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-16 14:37:13.757441	2026-07-16 14:42:15.347343	2026-07-16 14:37:13.757441	\N	book one	\N	\N
566	562	\N	\N	\N	\N	New Edition Title	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-16 14:42:15.35659	2026-07-16 14:42:15.362146	2026-07-16 14:42:15.35659	\N	new edition title	\N	\N
564	560	\N	\N	\N	\N	Book One	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-16 14:42:15.339339	2026-07-16 15:13:30.048961	2026-07-16 14:42:15.339339	\N	book one	\N	\N
539	535	\N	\N	\N	\N	Add Author Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-16 13:48:26.276186	2026-07-18 07:31:35.204262	2026-07-16 13:48:26.276186	\N	add author test	\N	\N
567	563	9780000000567	\N	\N	\N	Test Empty Strings	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-16 14:42:15.372357	2026-07-16 14:42:15.378628	2026-07-16 14:42:15.372357	\N	test empty strings	\N	\N
879	871	\N	\N	\N	\N	Test Book Part 2	eng	Self-published	2024	Self-published	0	\N	\N	Second test book created via API	manual	t	good	f	0	2026-07-21 16:09:00.984465	2026-07-21 16:09:00.984465	2026-07-21 16:09:00.984465	\N	test book part 2	\N	\N
568	564	97800000005681	\N	\N	\N	Corrupted ISBN Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-16 14:42:15.388905	2026-07-16 14:42:15.39602	2026-07-16 14:42:15.388905	\N	corrupted isbn test	\N	\N
569	565	\N	\N	\N	\N	Remove Authors Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-16 14:42:15.845192	2026-07-16 14:42:15.845192	2026-07-16 14:42:15.845192	\N	remove authors test	\N	\N
570	566	\N	\N	\N	\N	Remove Genres Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-16 14:42:15.864239	2026-07-16 14:42:15.864239	2026-07-16 14:42:15.864239	\N	remove genres test	\N	\N
571	567	\N	\N	\N	\N	Remove Tags Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-16 14:42:15.87988	2026-07-16 14:42:15.87988	2026-07-16 14:42:15.87988	\N	remove tags test	\N	\N
572	568	\N	\N	\N	\N	Nil Authors Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-16 14:42:15.895821	2026-07-16 14:42:15.895821	2026-07-16 14:42:15.895821	\N	nil authors test	\N	\N
573	569	\N	\N	\N	\N	Add Author Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-16 14:42:15.911775	2026-07-16 14:42:15.911775	2026-07-16 14:42:15.911775	\N	add author test	\N	\N
574	570	\N	\N	\N	\N	Test Book Part 1	eng	Self-published	2023	Self-published	0	\N	\N	First test book created via API	manual	t	good	f	0	2026-07-16 15:13:29.931365	2026-07-16 15:13:29.931365	2026-07-16 15:13:29.931365	\N	test book part 1	\N	\N
575	571	\N	\N	\N	\N	Test Book Part 2	eng	Self-published	2024	Self-published	0	\N	\N	Second test book created via API	manual	t	good	f	0	2026-07-16 15:13:29.939036	2026-07-16 15:13:29.939036	2026-07-16 15:13:29.939036	\N	test book part 2	\N	\N
1415	1407	\N	\N	\N	\N	New Edition Title	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-27 10:21:49.420491	2026-07-27 10:21:49.426541	2026-07-27 10:21:49.420491	\N	new edition title	\N	\N
576	572	\N	\N	\N	\N	Updated Book Title	eng	Self-published	2022	Self-published	0	\N	\N	Updated description	manual	t	good	f	0	2026-07-16 15:13:29.949942	2026-07-16 15:13:29.957681	2026-07-16 15:13:29.949942	\N	updated book title	\N	\N
578	574	\N	\N	\N	\N	Original Title	eng	Self-published	2025	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-16 15:13:29.986276	2026-07-16 15:13:29.994225	2026-07-16 15:13:29.986276	\N	original title	\N	\N
579	575	\N	\N	\N	\N	Book With ISBN Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-16 15:13:30.008294	2026-07-16 15:13:30.008294	2026-07-16 15:13:30.008294	\N	book with isbn test	\N	\N
580	576	\N	\N	\N	\N	Book Without ISBN	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-16 15:13:30.024446	2026-07-16 15:13:30.024446	2026-07-16 15:13:30.024446	\N	book without isbn	\N	\N
582	578	\N	\N	\N	\N	Book Two	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-16 15:13:30.046638	2026-07-16 15:13:30.046638	2026-07-16 15:13:30.046638	\N	book two	\N	\N
583	579	\N	\N	\N	\N	New Edition Title	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-16 15:13:30.058745	2026-07-16 15:13:30.06546	2026-07-16 15:13:30.058745	\N	new edition title	\N	\N
584	580	9780000000584	\N	\N	\N	Test Empty Strings	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-16 15:13:30.078098	2026-07-16 15:13:30.084311	2026-07-16 15:13:30.078098	\N	test empty strings	\N	\N
585	581	97800000005851	\N	\N	\N	Corrupted ISBN Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-16 15:13:30.093933	2026-07-16 15:13:30.100758	2026-07-16 15:13:30.093933	\N	corrupted isbn test	\N	\N
586	582	\N	\N	\N	\N	Remove Authors Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-16 15:13:30.521409	2026-07-16 15:13:30.521409	2026-07-16 15:13:30.521409	\N	remove authors test	\N	\N
587	583	\N	\N	\N	\N	Remove Genres Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-16 15:13:30.538622	2026-07-16 15:13:30.538622	2026-07-16 15:13:30.538622	\N	remove genres test	\N	\N
588	584	\N	\N	\N	\N	Remove Tags Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-16 15:13:30.556039	2026-07-16 15:13:30.556039	2026-07-16 15:13:30.556039	\N	remove tags test	\N	\N
589	585	\N	\N	\N	\N	Nil Authors Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-16 15:13:30.572756	2026-07-16 15:13:30.572756	2026-07-16 15:13:30.572756	\N	nil authors test	\N	\N
590	586	\N	\N	\N	\N	Add Author Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-16 15:13:30.589395	2026-07-16 15:13:30.589395	2026-07-16 15:13:30.589395	\N	add author test	\N	\N
591	587	\N	\N	\N	\N	Test Book Part 1	eng	Self-published	2023	Self-published	0	\N	\N	First test book created via API	manual	t	good	f	0	2026-07-16 15:18:54.200887	2026-07-16 15:18:54.200887	2026-07-16 15:18:54.200887	\N	test book part 1	\N	\N
592	588	\N	\N	\N	\N	Test Book Part 2	eng	Self-published	2024	Self-published	0	\N	\N	Second test book created via API	manual	t	good	f	0	2026-07-16 15:18:54.206965	2026-07-16 15:18:54.206965	2026-07-16 15:18:54.206965	\N	test book part 2	\N	\N
593	589	\N	\N	\N	\N	Updated Book Title	eng	Self-published	2022	Self-published	0	\N	\N	Updated description	manual	t	good	f	0	2026-07-16 15:18:54.218422	2026-07-16 15:18:54.224025	2026-07-16 15:18:54.218422	\N	updated book title	\N	\N
595	591	\N	\N	\N	\N	Original Title	eng	Self-published	2025	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-16 15:18:54.251429	2026-07-16 15:18:54.259364	2026-07-16 15:18:54.251429	\N	original title	\N	\N
596	592	\N	\N	\N	\N	Book With ISBN Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-16 15:18:54.272473	2026-07-16 15:18:54.272473	2026-07-16 15:18:54.272473	\N	book with isbn test	\N	\N
597	593	\N	\N	\N	\N	Book Without ISBN	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-16 15:18:54.290027	2026-07-16 15:18:54.290027	2026-07-16 15:18:54.290027	\N	book without isbn	\N	\N
599	595	\N	\N	\N	\N	Book Two	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-16 15:18:54.316896	2026-07-16 15:18:54.316896	2026-07-16 15:18:54.316896	\N	book two	\N	\N
581	577	\N	\N	\N	\N	Book One	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-16 15:13:30.040315	2026-07-16 15:18:54.319941	2026-07-16 15:13:30.040315	\N	book one	\N	\N
600	596	\N	\N	\N	\N	New Edition Title	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-16 15:18:54.328862	2026-07-16 15:18:54.334472	2026-07-16 15:18:54.328862	\N	new edition title	\N	\N
601	597	9780000000601	\N	\N	\N	Test Empty Strings	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-16 15:18:54.346092	2026-07-16 15:18:54.353069	2026-07-16 15:18:54.346092	\N	test empty strings	\N	\N
602	598	97800000006021	\N	\N	\N	Corrupted ISBN Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-16 15:18:54.366598	2026-07-16 15:18:54.372926	2026-07-16 15:18:54.366598	\N	corrupted isbn test	\N	\N
603	599	\N	\N	\N	\N	Remove Authors Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-16 15:18:54.800548	2026-07-16 15:18:54.800548	2026-07-16 15:18:54.800548	\N	remove authors test	\N	\N
604	600	\N	\N	\N	\N	Remove Genres Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-16 15:18:54.820132	2026-07-16 15:18:54.820132	2026-07-16 15:18:54.820132	\N	remove genres test	\N	\N
605	601	\N	\N	\N	\N	Remove Tags Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-16 15:18:54.83711	2026-07-16 15:18:54.83711	2026-07-16 15:18:54.83711	\N	remove tags test	\N	\N
606	602	\N	\N	\N	\N	Nil Authors Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-16 15:18:54.852558	2026-07-16 15:18:54.852558	2026-07-16 15:18:54.852558	\N	nil authors test	\N	\N
608	604	\N	\N	\N	\N	Test Book Part 1	eng	Self-published	2023	Self-published	0	\N	\N	First test book created via API	manual	t	good	f	0	2026-07-16 17:30:24.267049	2026-07-16 17:30:24.267049	2026-07-16 17:30:24.267049	\N	test book part 1	\N	\N
609	605	\N	\N	\N	\N	Test Book Part 2	eng	Self-published	2024	Self-published	0	\N	\N	Second test book created via API	manual	t	good	f	0	2026-07-16 17:30:24.272829	2026-07-16 17:30:24.272829	2026-07-16 17:30:24.272829	\N	test book part 2	\N	\N
610	606	\N	\N	\N	\N	Updated Book Title	eng	Self-published	2022	Self-published	0	\N	\N	Updated description	manual	t	good	f	0	2026-07-16 17:30:24.283475	2026-07-16 17:30:24.288888	2026-07-16 17:30:24.283475	\N	updated book title	\N	\N
612	608	\N	\N	\N	\N	Original Title	eng	Self-published	2025	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-16 17:30:24.31887	2026-07-16 17:30:24.326719	2026-07-16 17:30:24.31887	\N	original title	\N	\N
613	609	\N	\N	\N	\N	Book With ISBN Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-16 17:30:24.340765	2026-07-16 17:30:24.340765	2026-07-16 17:30:24.340765	\N	book with isbn test	\N	\N
614	610	\N	\N	\N	\N	Book Without ISBN	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-16 17:30:24.357448	2026-07-16 17:30:24.357448	2026-07-16 17:30:24.357448	\N	book without isbn	\N	\N
616	612	\N	\N	\N	\N	Book Two	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-16 17:30:24.378939	2026-07-16 17:30:24.378939	2026-07-16 17:30:24.378939	\N	book two	\N	\N
598	594	\N	\N	\N	\N	Book One	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-16 15:18:54.31107	2026-07-16 17:30:24.381036	2026-07-16 15:18:54.31107	\N	book one	\N	\N
617	613	\N	\N	\N	\N	New Edition Title	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-16 17:30:24.391772	2026-07-16 17:30:24.398602	2026-07-16 17:30:24.391772	\N	new edition title	\N	\N
618	614	9780000000618	\N	\N	\N	Test Empty Strings	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-16 17:30:24.40963	2026-07-16 17:30:24.415619	2026-07-16 17:30:24.40963	\N	test empty strings	\N	\N
619	615	97800000006191	\N	\N	\N	Corrupted ISBN Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-16 17:30:24.425615	2026-07-16 17:30:24.432624	2026-07-16 17:30:24.425615	\N	corrupted isbn test	\N	\N
620	616	\N	\N	\N	\N	Remove Authors Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-16 17:30:24.852068	2026-07-16 17:30:24.852068	2026-07-16 17:30:24.852068	\N	remove authors test	\N	\N
621	617	\N	\N	\N	\N	Remove Genres Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-16 17:30:24.868011	2026-07-16 17:30:24.868011	2026-07-16 17:30:24.868011	\N	remove genres test	\N	\N
622	618	\N	\N	\N	\N	Remove Tags Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-16 17:30:24.883889	2026-07-16 17:30:24.883889	2026-07-16 17:30:24.883889	\N	remove tags test	\N	\N
623	619	\N	\N	\N	\N	Nil Authors Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-16 17:30:24.901372	2026-07-16 17:30:24.901372	2026-07-16 17:30:24.901372	\N	nil authors test	\N	\N
624	620	\N	\N	\N	\N	Add Author Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-16 17:30:24.91708	2026-07-16 17:30:24.91708	2026-07-16 17:30:24.91708	\N	add author test	\N	\N
625	621	\N	\N	\N	\N	Test Book Part 1	eng	Self-published	2023	Self-published	0	\N	\N	First test book created via API	manual	t	good	f	0	2026-07-16 19:09:11.475866	2026-07-16 19:09:11.475866	2026-07-16 19:09:11.475866	\N	test book part 1	\N	\N
626	622	\N	\N	\N	\N	Test Book Part 2	eng	Self-published	2024	Self-published	0	\N	\N	Second test book created via API	manual	t	good	f	0	2026-07-16 19:09:11.482977	2026-07-16 19:09:11.482977	2026-07-16 19:09:11.482977	\N	test book part 2	\N	\N
627	623	\N	\N	\N	\N	Updated Book Title	eng	Self-published	2022	Self-published	0	\N	\N	Updated description	manual	t	good	f	0	2026-07-16 19:09:11.493664	2026-07-16 19:09:11.499599	2026-07-16 19:09:11.493664	\N	updated book title	\N	\N
629	625	\N	\N	\N	\N	Original Title	eng	Self-published	2025	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-16 19:09:11.531303	2026-07-16 19:09:11.540005	2026-07-16 19:09:11.531303	\N	original title	\N	\N
630	626	\N	\N	\N	\N	Book With ISBN Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-16 19:09:11.554085	2026-07-16 19:09:11.554085	2026-07-16 19:09:11.554085	\N	book with isbn test	\N	\N
631	627	\N	\N	\N	\N	Book Without ISBN	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-16 19:09:11.569106	2026-07-16 19:09:11.569106	2026-07-16 19:09:11.569106	\N	book without isbn	\N	\N
633	629	\N	\N	\N	\N	Book Two	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-16 19:09:11.590444	2026-07-16 19:09:11.590444	2026-07-16 19:09:11.590444	\N	book two	\N	\N
615	611	\N	\N	\N	\N	Book One	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-16 17:30:24.373031	2026-07-16 19:09:11.59294	2026-07-16 17:30:24.373031	\N	book one	\N	\N
634	630	\N	\N	\N	\N	New Edition Title	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-16 19:09:11.602721	2026-07-16 19:09:11.608808	2026-07-16 19:09:11.602721	\N	new edition title	\N	\N
635	631	9780000000635	\N	\N	\N	Test Empty Strings	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-16 19:09:11.619578	2026-07-16 19:09:11.626014	2026-07-16 19:09:11.619578	\N	test empty strings	\N	\N
636	632	97800000006361	\N	\N	\N	Corrupted ISBN Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-16 19:09:11.636018	2026-07-16 19:09:11.642689	2026-07-16 19:09:11.636018	\N	corrupted isbn test	\N	\N
637	633	\N	\N	\N	\N	Remove Authors Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-16 19:09:12.071822	2026-07-16 19:09:12.071822	2026-07-16 19:09:12.071822	\N	remove authors test	\N	\N
638	634	\N	\N	\N	\N	Remove Genres Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-16 19:09:12.088964	2026-07-16 19:09:12.088964	2026-07-16 19:09:12.088964	\N	remove genres test	\N	\N
639	635	\N	\N	\N	\N	Remove Tags Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-16 19:09:12.105906	2026-07-16 19:09:12.105906	2026-07-16 19:09:12.105906	\N	remove tags test	\N	\N
640	636	\N	\N	\N	\N	Nil Authors Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-16 19:09:12.124156	2026-07-16 19:09:12.124156	2026-07-16 19:09:12.124156	\N	nil authors test	\N	\N
641	637	\N	\N	\N	\N	Add Author Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-16 19:09:12.139744	2026-07-16 19:09:12.139744	2026-07-16 19:09:12.139744	\N	add author test	\N	\N
642	638	\N	\N	\N	\N	Test Book Part 1	eng	Self-published	2023	Self-published	0	\N	\N	First test book created via API	manual	t	good	f	0	2026-07-17 21:27:15.047921	2026-07-17 21:27:15.047921	2026-07-17 21:27:15.047921	\N	test book part 1	\N	\N
643	639	\N	\N	\N	\N	Test Book Part 2	eng	Self-published	2024	Self-published	0	\N	\N	Second test book created via API	manual	t	good	f	0	2026-07-17 21:27:15.054215	2026-07-17 21:27:15.054215	2026-07-17 21:27:15.054215	\N	test book part 2	\N	\N
644	640	\N	\N	\N	\N	Updated Book Title	eng	Self-published	2022	Self-published	0	\N	\N	Updated description	manual	t	good	f	0	2026-07-17 21:27:15.065208	2026-07-17 21:27:15.070967	2026-07-17 21:27:15.065208	\N	updated book title	\N	\N
646	642	\N	\N	\N	\N	Original Title	eng	Self-published	2025	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-17 21:27:15.094445	2026-07-17 21:27:15.102839	2026-07-17 21:27:15.094445	\N	original title	\N	\N
647	643	\N	\N	\N	\N	Book With ISBN Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-17 21:27:15.116195	2026-07-17 21:27:15.116195	2026-07-17 21:27:15.116195	\N	book with isbn test	\N	\N
648	644	\N	\N	\N	\N	Book Without ISBN	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-17 21:27:15.1349	2026-07-17 21:27:15.1349	2026-07-17 21:27:15.1349	\N	book without isbn	\N	\N
650	646	\N	\N	\N	\N	Book Two	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-17 21:27:15.153879	2026-07-17 21:27:15.153879	2026-07-17 21:27:15.153879	\N	book two	\N	\N
632	628	\N	\N	\N	\N	Book One	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-16 19:09:11.584764	2026-07-17 21:27:15.156632	2026-07-16 19:09:11.584764	\N	book one	\N	\N
651	647	\N	\N	\N	\N	New Edition Title	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-17 21:27:15.163861	2026-07-17 21:27:15.169347	2026-07-17 21:27:15.163861	\N	new edition title	\N	\N
652	648	9780000000652	\N	\N	\N	Test Empty Strings	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-17 21:27:15.178931	2026-07-17 21:27:15.184534	2026-07-17 21:27:15.178931	\N	test empty strings	\N	\N
653	649	97800000006531	\N	\N	\N	Corrupted ISBN Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-17 21:27:15.19212	2026-07-17 21:27:15.198456	2026-07-17 21:27:15.19212	\N	corrupted isbn test	\N	\N
654	650	\N	\N	\N	\N	Remove Authors Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-17 21:27:15.569617	2026-07-17 21:27:15.569617	2026-07-17 21:27:15.569617	\N	remove authors test	\N	\N
655	651	\N	\N	\N	\N	Remove Genres Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-17 21:27:15.585243	2026-07-17 21:27:15.585243	2026-07-17 21:27:15.585243	\N	remove genres test	\N	\N
656	652	\N	\N	\N	\N	Remove Tags Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-17 21:27:15.600539	2026-07-17 21:27:15.600539	2026-07-17 21:27:15.600539	\N	remove tags test	\N	\N
657	653	\N	\N	\N	\N	Nil Authors Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-17 21:27:15.616273	2026-07-17 21:27:15.616273	2026-07-17 21:27:15.616273	\N	nil authors test	\N	\N
658	654	\N	\N	\N	\N	Add Author Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-17 21:27:15.631743	2026-07-17 21:27:15.631743	2026-07-17 21:27:15.631743	\N	add author test	\N	\N
659	655	\N	\N	\N	\N	Test Book Part 1	eng	Self-published	2023	Self-published	0	\N	\N	First test book created via API	manual	t	good	f	0	2026-07-18 09:12:30.470262	2026-07-18 09:12:30.470262	2026-07-18 09:12:30.470262	\N	test book part 1	\N	\N
660	656	\N	\N	\N	\N	Test Book Part 2	eng	Self-published	2024	Self-published	0	\N	\N	Second test book created via API	manual	t	good	f	0	2026-07-18 09:12:30.486615	2026-07-18 09:12:30.486615	2026-07-18 09:12:30.486615	\N	test book part 2	\N	\N
661	657	\N	\N	\N	\N	Updated Book Title	eng	Self-published	2022	Self-published	0	\N	\N	Updated description	manual	t	good	f	0	2026-07-18 09:12:30.497268	2026-07-18 09:12:30.502395	2026-07-18 09:12:30.497268	\N	updated book title	\N	\N
880	872	\N	\N	\N	\N	Updated Book Title	eng	Self-published	2022	Self-published	0	\N	\N	Updated description	manual	t	good	f	0	2026-07-21 16:09:00.997458	2026-07-21 16:09:01.005409	2026-07-21 16:09:00.997458	\N	updated book title	\N	\N
887	879	\N	\N	\N	\N	New Edition Title	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-21 16:09:01.117888	2026-07-21 16:09:01.124218	2026-07-21 16:09:01.117888	\N	new edition title	\N	\N
663	659	\N	\N	\N	\N	Original Title	eng	Self-published	2025	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-18 09:12:30.539482	2026-07-18 09:12:30.547384	2026-07-18 09:12:30.539482	\N	original title	\N	\N
664	660	\N	\N	\N	\N	Book With ISBN Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-18 09:12:30.563162	2026-07-18 09:12:30.563162	2026-07-18 09:12:30.563162	\N	book with isbn test	\N	\N
665	661	\N	\N	\N	\N	Book Without ISBN	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-18 09:12:30.580551	2026-07-18 09:12:30.580551	2026-07-18 09:12:30.580551	\N	book without isbn	\N	\N
1416	1408	9780000001416	\N	\N	\N	Test Empty Strings	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-27 10:21:49.438	2026-07-27 10:21:49.444863	2026-07-27 10:21:49.438	\N	test empty strings	\N	\N
667	663	\N	\N	\N	\N	Book Two	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-18 09:12:30.603254	2026-07-18 09:12:30.603254	2026-07-18 09:12:30.603254	\N	book two	\N	\N
649	645	\N	\N	\N	\N	Book One	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-17 21:27:15.148364	2026-07-18 09:12:30.605313	2026-07-17 21:27:15.148364	\N	book one	\N	\N
668	664	\N	\N	\N	\N	New Edition Title	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-18 09:12:30.615992	2026-07-18 09:12:30.621677	2026-07-18 09:12:30.615992	\N	new edition title	\N	\N
669	665	9780000000669	\N	\N	\N	Test Empty Strings	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-18 09:12:30.632468	2026-07-18 09:12:30.638615	2026-07-18 09:12:30.632468	\N	test empty strings	\N	\N
670	666	97800000006701	\N	\N	\N	Corrupted ISBN Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-18 09:12:30.649945	2026-07-18 09:12:30.656627	2026-07-18 09:12:30.649945	\N	corrupted isbn test	\N	\N
671	667	\N	\N	\N	\N	Remove Authors Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-18 09:12:31.131251	2026-07-18 09:12:31.131251	2026-07-18 09:12:31.131251	\N	remove authors test	\N	\N
672	668	\N	\N	\N	\N	Remove Genres Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-18 09:12:31.150943	2026-07-18 09:12:31.150943	2026-07-18 09:12:31.150943	\N	remove genres test	\N	\N
673	669	\N	\N	\N	\N	Remove Tags Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-18 09:12:31.167339	2026-07-18 09:12:31.167339	2026-07-18 09:12:31.167339	\N	remove tags test	\N	\N
674	670	\N	\N	\N	\N	Nil Authors Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-18 09:12:31.186479	2026-07-18 09:12:31.186479	2026-07-18 09:12:31.186479	\N	nil authors test	\N	\N
676	672	\N	\N	\N	\N	Паразиты сознания	rus		\N	\N	\N	\N	\N	\N	imported	t	good	f	0	2026-07-20 10:38:30.159804	2026-07-20 10:38:30.159804	2026-07-20 10:38:30.159804	\N	паразиты сознания	\N	\N
677	673	\N	\N	\N	\N	КРАСНАЯ КНИГА	rus		2010	\N	\N	\N	\N	\N	imported	t	good	f	0	2026-07-20 12:09:39.385954	2026-07-20 12:09:39.385954	2026-07-20 12:09:39.385954	\N	красная книга	\N	\N
678	674	\N	\N	\N	\N	Test Book Part 1	eng	Self-published	2023	Self-published	0	\N	\N	First test book created via API	manual	t	good	f	0	2026-07-20 15:01:22.430491	2026-07-20 15:01:22.430491	2026-07-20 15:01:22.430491	\N	test book part 1	\N	\N
679	675	\N	\N	\N	\N	Test Book Part 2	eng	Self-published	2024	Self-published	0	\N	\N	Second test book created via API	manual	t	good	f	0	2026-07-20 15:01:22.436108	2026-07-20 15:01:22.436108	2026-07-20 15:01:22.436108	\N	test book part 2	\N	\N
680	676	\N	\N	\N	\N	Updated Book Title	eng	Self-published	2022	Self-published	0	\N	\N	Updated description	manual	t	good	f	0	2026-07-20 15:01:22.447129	2026-07-20 15:01:22.452506	2026-07-20 15:01:22.447129	\N	updated book title	\N	\N
682	678	\N	\N	\N	\N	Original Title	eng	Self-published	2025	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-20 15:01:22.481121	2026-07-20 15:01:22.489485	2026-07-20 15:01:22.481121	\N	original title	\N	\N
683	679	\N	\N	\N	\N	Book With ISBN Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-20 15:01:22.506192	2026-07-20 15:01:22.506192	2026-07-20 15:01:22.506192	\N	book with isbn test	\N	\N
684	680	\N	\N	\N	\N	Book Without ISBN	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-20 15:01:22.526038	2026-07-20 15:01:22.526038	2026-07-20 15:01:22.526038	\N	book without isbn	\N	\N
686	682	\N	\N	\N	\N	Book Two	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-20 15:01:22.548669	2026-07-20 15:01:22.548669	2026-07-20 15:01:22.548669	\N	book two	\N	\N
666	662	\N	\N	\N	\N	Book One	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-18 09:12:30.597838	2026-07-20 15:01:22.551123	2026-07-18 09:12:30.597838	\N	book one	\N	\N
687	683	\N	\N	\N	\N	New Edition Title	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-20 15:01:22.562157	2026-07-20 15:01:22.570017	2026-07-20 15:01:22.562157	\N	new edition title	\N	\N
688	684	9780000000688	\N	\N	\N	Test Empty Strings	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-20 15:01:22.582043	2026-07-20 15:01:22.588036	2026-07-20 15:01:22.582043	\N	test empty strings	\N	\N
689	685	97800000006891	\N	\N	\N	Corrupted ISBN Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-20 15:01:22.601465	2026-07-20 15:01:22.607576	2026-07-20 15:01:22.601465	\N	corrupted isbn test	\N	\N
690	686	\N	\N	\N	\N	Remove Authors Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-20 15:01:23.10249	2026-07-20 15:01:23.10249	2026-07-20 15:01:23.10249	\N	remove authors test	\N	\N
691	687	\N	\N	\N	\N	Remove Genres Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-20 15:01:23.120299	2026-07-20 15:01:23.120299	2026-07-20 15:01:23.120299	\N	remove genres test	\N	\N
692	688	\N	\N	\N	\N	Remove Tags Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-20 15:01:23.13512	2026-07-20 15:01:23.13512	2026-07-20 15:01:23.13512	\N	remove tags test	\N	\N
693	689	\N	\N	\N	\N	Nil Authors Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-20 15:01:23.154087	2026-07-20 15:01:23.154087	2026-07-20 15:01:23.154087	\N	nil authors test	\N	\N
695	691	\N	\N	\N	\N	Test Book Part 1	eng	Self-published	2023	Self-published	0	\N	\N	First test book created via API	manual	t	good	f	0	2026-07-20 17:32:12.962153	2026-07-20 17:32:12.962153	2026-07-20 17:32:12.962153	\N	test book part 1	\N	\N
696	692	\N	\N	\N	\N	Test Book Part 2	eng	Self-published	2024	Self-published	0	\N	\N	Second test book created via API	manual	t	good	f	0	2026-07-20 17:32:12.967865	2026-07-20 17:32:12.967865	2026-07-20 17:32:12.967865	\N	test book part 2	\N	\N
697	693	\N	\N	\N	\N	Updated Book Title	eng	Self-published	2022	Self-published	0	\N	\N	Updated description	manual	t	good	f	0	2026-07-20 17:32:12.980678	2026-07-20 17:32:12.986489	2026-07-20 17:32:12.980678	\N	updated book title	\N	\N
699	695	\N	\N	\N	\N	Original Title	eng	Self-published	2025	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-20 17:32:13.018689	2026-07-20 17:32:13.026193	2026-07-20 17:32:13.018689	\N	original title	\N	\N
700	696	\N	\N	\N	\N	Book With ISBN Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-20 17:32:13.044077	2026-07-20 17:32:13.044077	2026-07-20 17:32:13.044077	\N	book with isbn test	\N	\N
701	697	\N	\N	\N	\N	Book Without ISBN	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-20 17:32:13.062034	2026-07-20 17:32:13.062034	2026-07-20 17:32:13.062034	\N	book without isbn	\N	\N
703	699	\N	\N	\N	\N	Book Two	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-20 17:32:13.083713	2026-07-20 17:32:13.083713	2026-07-20 17:32:13.083713	\N	book two	\N	\N
685	681	\N	\N	\N	\N	Book One	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-20 15:01:22.543141	2026-07-20 17:32:13.086153	2026-07-20 15:01:22.543141	\N	book one	\N	\N
702	698	\N	\N	\N	\N	Book One	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-20 17:32:13.078359	2026-07-20 18:45:53.468065	2026-07-20 17:32:13.078359	\N	book one	\N	\N
704	700	\N	\N	\N	\N	New Edition Title	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-20 17:32:13.10142	2026-07-20 17:32:13.107834	2026-07-20 17:32:13.10142	\N	new edition title	\N	\N
705	701	9780000000705	\N	\N	\N	Test Empty Strings	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-20 17:32:13.118734	2026-07-20 17:32:13.125011	2026-07-20 17:32:13.118734	\N	test empty strings	\N	\N
888	880	9780000000888	\N	\N	\N	Test Empty Strings	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-21 16:09:01.141347	2026-07-21 16:09:01.147385	2026-07-21 16:09:01.141347	\N	test empty strings	\N	\N
706	702	97800000007061	\N	\N	\N	Corrupted ISBN Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-20 17:32:13.136817	2026-07-20 17:32:13.143567	2026-07-20 17:32:13.136817	\N	corrupted isbn test	\N	\N
707	703	\N	\N	\N	\N	Remove Authors Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-20 17:32:13.612781	2026-07-20 17:32:13.612781	2026-07-20 17:32:13.612781	\N	remove authors test	\N	\N
708	704	\N	\N	\N	\N	Remove Genres Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-20 17:32:13.63024	2026-07-20 17:32:13.63024	2026-07-20 17:32:13.63024	\N	remove genres test	\N	\N
709	705	\N	\N	\N	\N	Remove Tags Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-20 17:32:13.648537	2026-07-20 17:32:13.648537	2026-07-20 17:32:13.648537	\N	remove tags test	\N	\N
710	706	\N	\N	\N	\N	Nil Authors Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-20 17:32:13.664145	2026-07-20 17:32:13.664145	2026-07-20 17:32:13.664145	\N	nil authors test	\N	\N
711	707	\N	\N	\N	\N	Add Author Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-20 17:32:13.680968	2026-07-20 17:32:13.680968	2026-07-20 17:32:13.680968	\N	add author test	\N	\N
712	708	\N	\N	\N	\N	Test Book Part 1	eng	Self-published	2023	Self-published	0	\N	\N	First test book created via API	manual	t	good	f	0	2026-07-20 18:45:53.345029	2026-07-20 18:45:53.345029	2026-07-20 18:45:53.345029	\N	test book part 1	\N	\N
713	709	\N	\N	\N	\N	Test Book Part 2	eng	Self-published	2024	Self-published	0	\N	\N	Second test book created via API	manual	t	good	f	0	2026-07-20 18:45:53.351772	2026-07-20 18:45:53.351772	2026-07-20 18:45:53.351772	\N	test book part 2	\N	\N
891	883	\N	\N	\N	\N	Remove Genres Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-21 16:09:01.677387	2026-07-21 16:09:01.677387	2026-07-21 16:09:01.677387	\N	remove genres test	\N	\N
714	710	\N	\N	\N	\N	Updated Book Title	eng	Self-published	2022	Self-published	0	\N	\N	Updated description	manual	t	good	f	0	2026-07-20 18:45:53.362712	2026-07-20 18:45:53.368835	2026-07-20 18:45:53.362712	\N	updated book title	\N	\N
728	724	\N	\N	\N	\N	Add Author Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-20 18:45:54.045814	2026-07-24 13:21:32.09767	2026-07-20 18:45:54.045814	\N	add author test	\N	\N
716	712	\N	\N	\N	\N	Original Title	eng	Self-published	2025	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-20 18:45:53.398766	2026-07-20 18:45:53.406624	2026-07-20 18:45:53.398766	\N	original title	\N	\N
717	713	\N	\N	\N	\N	Book With ISBN Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-20 18:45:53.420315	2026-07-20 18:45:53.420315	2026-07-20 18:45:53.420315	\N	book with isbn test	\N	\N
718	714	\N	\N	\N	\N	Book Without ISBN	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-20 18:45:53.441716	2026-07-20 18:45:53.441716	2026-07-20 18:45:53.441716	\N	book without isbn	\N	\N
720	716	\N	\N	\N	\N	Book Two	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-20 18:45:53.465728	2026-07-20 18:45:53.465728	2026-07-20 18:45:53.465728	\N	book two	\N	\N
721	717	\N	\N	\N	\N	New Edition Title	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-20 18:45:53.478085	2026-07-20 18:45:53.48407	2026-07-20 18:45:53.478085	\N	new edition title	\N	\N
722	718	9780000000722	\N	\N	\N	Test Empty Strings	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-20 18:45:53.500753	2026-07-20 18:45:53.506407	2026-07-20 18:45:53.500753	\N	test empty strings	\N	\N
723	719	97800000007231	\N	\N	\N	Corrupted ISBN Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-20 18:45:53.516949	2026-07-20 18:45:53.523416	2026-07-20 18:45:53.516949	\N	corrupted isbn test	\N	\N
724	720	\N	\N	\N	\N	Remove Authors Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-20 18:45:53.974118	2026-07-20 18:45:53.974118	2026-07-20 18:45:53.974118	\N	remove authors test	\N	\N
725	721	\N	\N	\N	\N	Remove Genres Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-20 18:45:53.994799	2026-07-20 18:45:53.994799	2026-07-20 18:45:53.994799	\N	remove genres test	\N	\N
726	722	\N	\N	\N	\N	Remove Tags Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-20 18:45:54.013938	2026-07-20 18:45:54.013938	2026-07-20 18:45:54.013938	\N	remove tags test	\N	\N
727	723	\N	\N	\N	\N	Nil Authors Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-20 18:45:54.030519	2026-07-20 18:45:54.030519	2026-07-20 18:45:54.030519	\N	nil authors test	\N	\N
729	725	\N	\N	\N	\N	Test Book Part 1	eng	Self-published	2023	Self-published	0	\N	\N	First test book created via API	manual	t	good	f	0	2026-07-21 13:08:39.385614	2026-07-21 13:08:39.385614	2026-07-21 13:08:39.385614	\N	test book part 1	\N	\N
730	726	\N	\N	\N	\N	Test Book Part 2	eng	Self-published	2024	Self-published	0	\N	\N	Second test book created via API	manual	t	good	f	0	2026-07-21 13:08:39.396844	2026-07-21 13:08:39.396844	2026-07-21 13:08:39.396844	\N	test book part 2	\N	\N
731	727	\N	\N	\N	\N	Updated Book Title	eng	Self-published	2022	Self-published	0	\N	\N	Updated description	manual	t	good	f	0	2026-07-21 13:08:39.409464	2026-07-21 13:08:39.4162	2026-07-21 13:08:39.409464	\N	updated book title	\N	\N
733	729	\N	\N	\N	\N	Original Title	eng	Self-published	2025	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-21 13:08:39.449785	2026-07-21 13:08:39.457612	2026-07-21 13:08:39.449785	\N	original title	\N	\N
734	730	\N	\N	\N	\N	Book With ISBN Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-21 13:08:39.471339	2026-07-21 13:08:39.471339	2026-07-21 13:08:39.471339	\N	book with isbn test	\N	\N
735	731	\N	\N	\N	\N	Book Without ISBN	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-21 13:08:39.488401	2026-07-21 13:08:39.488401	2026-07-21 13:08:39.488401	\N	book without isbn	\N	\N
737	733	\N	\N	\N	\N	Book Two	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-21 13:08:39.510116	2026-07-21 13:08:39.510116	2026-07-21 13:08:39.510116	\N	book two	\N	\N
719	715	\N	\N	\N	\N	Book One	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-20 18:45:53.460612	2026-07-21 13:08:39.512235	2026-07-20 18:45:53.460612	\N	book one	\N	\N
738	734	\N	\N	\N	\N	New Edition Title	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-21 13:08:39.524471	2026-07-21 13:08:39.530424	2026-07-21 13:08:39.524471	\N	new edition title	\N	\N
739	735	9780000000739	\N	\N	\N	Test Empty Strings	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-21 13:08:39.541718	2026-07-21 13:08:39.547819	2026-07-21 13:08:39.541718	\N	test empty strings	\N	\N
740	736	97800000007401	\N	\N	\N	Corrupted ISBN Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-21 13:08:39.560093	2026-07-21 13:08:39.566643	2026-07-21 13:08:39.560093	\N	corrupted isbn test	\N	\N
741	737	\N	\N	\N	\N	Remove Authors Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-21 13:08:40.01312	2026-07-21 13:08:40.01312	2026-07-21 13:08:40.01312	\N	remove authors test	\N	\N
742	738	\N	\N	\N	\N	Remove Genres Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-21 13:08:40.030444	2026-07-21 13:08:40.030444	2026-07-21 13:08:40.030444	\N	remove genres test	\N	\N
743	739	\N	\N	\N	\N	Remove Tags Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-21 13:08:40.048555	2026-07-21 13:08:40.048555	2026-07-21 13:08:40.048555	\N	remove tags test	\N	\N
744	740	\N	\N	\N	\N	Nil Authors Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-21 13:08:40.064763	2026-07-21 13:08:40.064763	2026-07-21 13:08:40.064763	\N	nil authors test	\N	\N
745	741	\N	\N	\N	\N	Add Author Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-21 13:08:40.083217	2026-07-21 13:08:40.083217	2026-07-21 13:08:40.083217	\N	add author test	\N	\N
746	742	\N	\N	\N	\N	Test Book Part 1	eng	Self-published	2023	Self-published	0	\N	\N	First test book created via API	manual	t	good	f	0	2026-07-21 14:59:09.716402	2026-07-21 14:59:09.716402	2026-07-21 14:59:09.716402	\N	test book part 1	\N	\N
747	743	\N	\N	\N	\N	Test Book Part 2	eng	Self-published	2024	Self-published	0	\N	\N	Second test book created via API	manual	t	good	f	0	2026-07-21 14:59:09.723218	2026-07-21 14:59:09.723218	2026-07-21 14:59:09.723218	\N	test book part 2	\N	\N
748	744	\N	\N	\N	\N	Updated Book Title	eng	Self-published	2022	Self-published	0	\N	\N	Updated description	manual	t	good	f	0	2026-07-21 14:59:09.734982	2026-07-21 14:59:09.740859	2026-07-21 14:59:09.734982	\N	updated book title	\N	\N
882	874	\N	\N	\N	\N	Original Title	eng	Self-published	2025	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-21 16:09:01.036043	2026-07-21 16:09:01.044586	2026-07-21 16:09:01.036043	\N	original title	\N	\N
770	766	\N	\N	\N	\N	Book One	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-21 15:40:16.178137	2026-07-21 16:09:01.104082	2026-07-21 15:40:16.178137	\N	book one	\N	\N
750	746	\N	\N	\N	\N	Original Title	eng	Self-published	2025	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-21 14:59:09.77278	2026-07-21 14:59:09.782455	2026-07-21 14:59:09.77278	\N	original title	\N	\N
751	747	\N	\N	\N	\N	Book With ISBN Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-21 14:59:09.797791	2026-07-21 14:59:09.797791	2026-07-21 14:59:09.797791	\N	book with isbn test	\N	\N
752	748	\N	\N	\N	\N	Book Without ISBN	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-21 14:59:09.814749	2026-07-21 14:59:09.814749	2026-07-21 14:59:09.814749	\N	book without isbn	\N	\N
754	750	\N	\N	\N	\N	Book Two	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-21 14:59:09.837351	2026-07-21 14:59:09.837351	2026-07-21 14:59:09.837351	\N	book two	\N	\N
736	732	\N	\N	\N	\N	Book One	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-21 13:08:39.504794	2026-07-21 14:59:09.83987	2026-07-21 13:08:39.504794	\N	book one	\N	\N
889	881	97800000008891	\N	\N	\N	Corrupted ISBN Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-21 16:09:01.15954	2026-07-21 16:09:01.166031	2026-07-21 16:09:01.15954	\N	corrupted isbn test	\N	\N
892	884	\N	\N	\N	\N	Remove Tags Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-21 16:09:01.69528	2026-07-21 16:09:01.69528	2026-07-21 16:09:01.69528	\N	remove tags test	\N	\N
755	751	\N	\N	\N	\N	New Edition Title	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-21 14:59:09.853609	2026-07-21 14:59:09.861344	2026-07-21 14:59:09.853609	\N	new edition title	\N	\N
895	887	\N	\N	\N	\N	Test Book Part 1	eng	Self-published	2023	Self-published	0	\N	\N	First test book created via API	manual	t	good	f	0	2026-07-21 20:55:27.680959	2026-07-21 20:55:27.680959	2026-07-21 20:55:27.680959	\N	test book part 1	\N	\N
756	752	9780000000756	\N	\N	\N	Test Empty Strings	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-21 14:59:09.874288	2026-07-21 14:59:09.882011	2026-07-21 14:59:09.874288	\N	test empty strings	\N	\N
896	888	\N	\N	\N	\N	Test Book Part 2	eng	Self-published	2024	Self-published	0	\N	\N	Second test book created via API	manual	t	good	f	0	2026-07-21 20:55:27.687234	2026-07-21 20:55:27.687234	2026-07-21 20:55:27.687234	\N	test book part 2	\N	\N
757	753	97800000007571	\N	\N	\N	Corrupted ISBN Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-21 14:59:09.896072	2026-07-21 14:59:09.902984	2026-07-21 14:59:09.896072	\N	corrupted isbn test	\N	\N
758	754	\N	\N	\N	\N	Remove Authors Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-21 14:59:10.375155	2026-07-21 14:59:10.375155	2026-07-21 14:59:10.375155	\N	remove authors test	\N	\N
759	755	\N	\N	\N	\N	Remove Genres Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-21 14:59:10.391754	2026-07-21 14:59:10.391754	2026-07-21 14:59:10.391754	\N	remove genres test	\N	\N
760	756	\N	\N	\N	\N	Remove Tags Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-21 14:59:10.41118	2026-07-21 14:59:10.41118	2026-07-21 14:59:10.41118	\N	remove tags test	\N	\N
761	757	\N	\N	\N	\N	Nil Authors Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-21 14:59:10.428556	2026-07-21 14:59:10.428556	2026-07-21 14:59:10.428556	\N	nil authors test	\N	\N
762	758	\N	\N	\N	\N	Add Author Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-21 14:59:10.443922	2026-07-21 14:59:10.443922	2026-07-21 14:59:10.443922	\N	add author test	\N	\N
763	759	\N	\N	\N	\N	Test Book Part 1	eng	Self-published	2023	Self-published	0	\N	\N	First test book created via API	manual	t	good	f	0	2026-07-21 15:40:16.044542	2026-07-21 15:40:16.044542	2026-07-21 15:40:16.044542	\N	test book part 1	\N	\N
764	760	\N	\N	\N	\N	Test Book Part 2	eng	Self-published	2024	Self-published	0	\N	\N	Second test book created via API	manual	t	good	f	0	2026-07-21 15:40:16.051771	2026-07-21 15:40:16.051771	2026-07-21 15:40:16.051771	\N	test book part 2	\N	\N
765	761	\N	\N	\N	\N	Updated Book Title	eng	Self-published	2022	Self-published	0	\N	\N	Updated description	manual	t	good	f	0	2026-07-21 15:40:16.066138	2026-07-21 15:40:16.072596	2026-07-21 15:40:16.066138	\N	updated book title	\N	\N
901	893	\N	\N	\N	\N	Book Without ISBN	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-21 20:55:27.775829	2026-07-21 20:55:27.775829	2026-07-21 20:55:27.775829	\N	book without isbn	\N	\N
767	763	\N	\N	\N	\N	Original Title	eng	Self-published	2025	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-21 15:40:16.112249	2026-07-21 15:40:16.122076	2026-07-21 15:40:16.112249	\N	original title	\N	\N
768	764	\N	\N	\N	\N	Book With ISBN Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-21 15:40:16.138729	2026-07-21 15:40:16.138729	2026-07-21 15:40:16.138729	\N	book with isbn test	\N	\N
769	765	\N	\N	\N	\N	Book Without ISBN	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-21 15:40:16.160175	2026-07-21 15:40:16.160175	2026-07-21 15:40:16.160175	\N	book without isbn	\N	\N
904	896	\N	\N	\N	\N	New Edition Title	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-21 20:55:27.811507	2026-07-21 20:55:27.817196	2026-07-21 20:55:27.811507	\N	new edition title	\N	\N
771	767	\N	\N	\N	\N	Book Two	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-21 15:40:16.184526	2026-07-21 15:40:16.184526	2026-07-21 15:40:16.184526	\N	book two	\N	\N
753	749	\N	\N	\N	\N	Book One	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-21 14:59:09.831287	2026-07-21 15:40:16.187746	2026-07-21 14:59:09.831287	\N	book one	\N	\N
772	768	\N	\N	\N	\N	New Edition Title	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-21 15:40:16.20389	2026-07-21 15:40:16.210411	2026-07-21 15:40:16.20389	\N	new edition title	\N	\N
906	898	97800000009061	\N	\N	\N	Corrupted ISBN Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-21 20:55:27.846384	2026-07-21 20:55:27.852963	2026-07-21 20:55:27.846384	\N	corrupted isbn test	\N	\N
773	769	9780000000773	\N	\N	\N	Test Empty Strings	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-21 15:40:16.22509	2026-07-21 15:40:16.23154	2026-07-21 15:40:16.22509	\N	test empty strings	\N	\N
908	900	\N	\N	\N	\N	Remove Genres Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-21 20:55:28.325034	2026-07-21 20:55:28.325034	2026-07-21 20:55:28.325034	\N	remove genres test	\N	\N
774	770	97800000007741	\N	\N	\N	Corrupted ISBN Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-21 15:40:16.244406	2026-07-21 15:40:16.252715	2026-07-21 15:40:16.244406	\N	corrupted isbn test	\N	\N
775	771	\N	\N	\N	\N	Remove Authors Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-21 15:40:16.874415	2026-07-21 15:40:16.874415	2026-07-21 15:40:16.874415	\N	remove authors test	\N	\N
776	772	\N	\N	\N	\N	Remove Genres Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-21 15:40:16.896279	2026-07-21 15:40:16.896279	2026-07-21 15:40:16.896279	\N	remove genres test	\N	\N
777	773	\N	\N	\N	\N	Remove Tags Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-21 15:40:16.915469	2026-07-21 15:40:16.915469	2026-07-21 15:40:16.915469	\N	remove tags test	\N	\N
778	774	\N	\N	\N	\N	Nil Authors Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-21 15:40:16.941188	2026-07-21 15:40:16.941188	2026-07-21 15:40:16.941188	\N	nil authors test	\N	\N
779	775	\N	\N	\N	\N	Add Author Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-21 15:40:16.960577	2026-07-21 15:40:16.960577	2026-07-21 15:40:16.960577	\N	add author test	\N	\N
910	902	\N	\N	\N	\N	Nil Authors Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-21 20:55:28.358562	2026-07-21 20:55:28.358562	2026-07-21 20:55:28.358562	\N	nil authors test	\N	\N
912	904	\N	\N	\N	\N	Test Book Part 1	eng	Self-published	2023	Self-published	0	\N	\N	First test book created via API	manual	t	good	f	0	2026-07-21 21:23:05.812351	2026-07-21 21:23:05.812351	2026-07-21 21:23:05.812351	\N	test book part 1	\N	\N
913	905	\N	\N	\N	\N	Test Book Part 2	eng	Self-published	2024	Self-published	0	\N	\N	Second test book created via API	manual	t	good	f	0	2026-07-21 21:23:05.818941	2026-07-21 21:23:05.818941	2026-07-21 21:23:05.818941	\N	test book part 2	\N	\N
917	909	\N	\N	\N	\N	Book With ISBN Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-21 21:23:05.89653	2026-07-21 21:23:05.89653	2026-07-21 21:23:05.89653	\N	book with isbn test	\N	\N
920	912	\N	\N	\N	\N	Book Two	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-21 21:23:05.941308	2026-07-21 21:23:05.941308	2026-07-21 21:23:05.941308	\N	book two	\N	\N
931	923	\N	\N	\N	\N	Updated Book Title	eng	Self-published	2022	Self-published	0	\N	\N	Updated description	manual	t	good	f	0	2026-07-22 09:33:47.73352	2026-07-22 09:33:47.740717	2026-07-22 09:33:47.73352	\N	updated book title	\N	\N
933	925	\N	\N	\N	\N	Original Title	eng	Self-published	2025	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-22 09:33:47.774321	2026-07-22 09:33:47.782504	2026-07-22 09:33:47.774321	\N	original title	\N	\N
934	926	\N	\N	\N	\N	Book With ISBN Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-22 09:33:47.79992	2026-07-22 09:33:47.79992	2026-07-22 09:33:47.79992	\N	book with isbn test	\N	\N
935	927	\N	\N	\N	\N	Book Without ISBN	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-22 09:33:47.82124	2026-07-22 09:33:47.82124	2026-07-22 09:33:47.82124	\N	book without isbn	\N	\N
937	929	\N	\N	\N	\N	Book Two	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-22 09:33:47.847151	2026-07-22 09:33:47.847151	2026-07-22 09:33:47.847151	\N	book two	\N	\N
938	930	\N	\N	\N	\N	New Edition Title	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-22 09:33:47.863741	2026-07-22 09:33:47.869647	2026-07-22 09:33:47.863741	\N	new edition title	\N	\N
939	931	9780000000939	\N	\N	\N	Test Empty Strings	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-22 09:33:47.888197	2026-07-22 09:33:47.894348	2026-07-22 09:33:47.888197	\N	test empty strings	\N	\N
940	932	97800000009401	\N	\N	\N	Corrupted ISBN Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-22 09:33:52.900853	2026-07-22 09:33:52.906945	2026-07-22 09:33:52.900853	\N	corrupted isbn test	\N	\N
941	933	\N	\N	\N	\N	Remove Authors Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-22 09:33:53.384278	2026-07-22 09:33:53.384278	2026-07-22 09:33:53.384278	\N	remove authors test	\N	\N
942	934	\N	\N	\N	\N	Remove Genres Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-22 09:33:53.402597	2026-07-22 09:33:53.402597	2026-07-22 09:33:53.402597	\N	remove genres test	\N	\N
943	935	\N	\N	\N	\N	Remove Tags Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-22 09:33:53.426572	2026-07-22 09:33:53.426572	2026-07-22 09:33:53.426572	\N	remove tags test	\N	\N
944	936	\N	\N	\N	\N	Nil Authors Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-22 09:33:53.443721	2026-07-22 09:33:53.443721	2026-07-22 09:33:53.443721	\N	nil authors test	\N	\N
945	937	\N	\N	\N	\N	Add Author Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-22 09:33:53.460587	2026-07-22 09:33:53.460587	2026-07-22 09:33:53.460587	\N	add author test	\N	\N
966	958	\N	\N	\N	\N	Updated Book Title	eng	Self-published	2022	Self-published	0	\N	\N	Updated description	manual	t	good	f	0	2026-07-24 13:13:24.58461	2026-07-24 13:13:24.590634	2026-07-24 13:13:24.58461	\N	updated book title	\N	\N
947	939	\N	\N	\N	\N	Test Book Part 1	eng	Self-published	2023	Self-published	0	\N	\N	First test book created via API	manual	t	good	f	0	2026-07-23 07:17:39.553704	2026-07-23 07:17:39.553704	2026-07-23 07:17:39.553704	\N	test book part 1	\N	\N
948	940	\N	\N	\N	\N	Test Book Part 2	eng	Self-published	2024	Self-published	0	\N	\N	Second test book created via API	manual	t	good	f	0	2026-07-23 07:17:39.561426	2026-07-23 07:17:39.561426	2026-07-23 07:17:39.561426	\N	test book part 2	\N	\N
949	941	\N	\N	\N	\N	Updated Book Title	eng	Self-published	2022	Self-published	0	\N	\N	Updated description	manual	t	good	f	0	2026-07-23 07:17:39.577495	2026-07-23 07:17:39.584045	2026-07-23 07:17:39.577495	\N	updated book title	\N	\N
951	943	\N	\N	\N	\N	Original Title	eng	Self-published	2025	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-23 07:17:39.613987	2026-07-23 07:17:39.624169	2026-07-23 07:17:39.613987	\N	original title	\N	\N
952	944	\N	\N	\N	\N	Book With ISBN Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-23 07:17:39.63965	2026-07-23 07:17:39.63965	2026-07-23 07:17:39.63965	\N	book with isbn test	\N	\N
953	945	\N	\N	\N	\N	Book Without ISBN	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-23 07:17:39.658919	2026-07-23 07:17:39.658919	2026-07-23 07:17:39.658919	\N	book without isbn	\N	\N
955	947	\N	\N	\N	\N	Book Two	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-23 07:17:39.681092	2026-07-23 07:17:39.681092	2026-07-23 07:17:39.681092	\N	book two	\N	\N
936	928	\N	\N	\N	\N	Book One	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-22 09:33:47.840991	2026-07-23 07:17:39.683644	2026-07-22 09:33:47.840991	\N	book one	\N	\N
956	948	\N	\N	\N	\N	New Edition Title	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-23 07:17:39.694974	2026-07-23 07:17:39.700855	2026-07-23 07:17:39.694974	\N	new edition title	\N	\N
957	949	9780000000957	\N	\N	\N	Test Empty Strings	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-23 07:17:39.716267	2026-07-23 07:17:39.722537	2026-07-23 07:17:39.716267	\N	test empty strings	\N	\N
958	950	97800000009581	\N	\N	\N	Corrupted ISBN Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-23 07:17:39.733793	2026-07-23 07:17:39.740808	2026-07-23 07:17:39.733793	\N	corrupted isbn test	\N	\N
959	951	\N	\N	\N	\N	Remove Authors Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-23 07:17:40.237189	2026-07-23 07:17:40.237189	2026-07-23 07:17:40.237189	\N	remove authors test	\N	\N
960	952	\N	\N	\N	\N	Remove Genres Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-23 07:17:40.254592	2026-07-23 07:17:40.254592	2026-07-23 07:17:40.254592	\N	remove genres test	\N	\N
961	953	\N	\N	\N	\N	Remove Tags Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-23 07:17:40.272656	2026-07-23 07:17:40.272656	2026-07-23 07:17:40.272656	\N	remove tags test	\N	\N
962	954	\N	\N	\N	\N	Nil Authors Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-23 07:17:40.289769	2026-07-23 07:17:40.289769	2026-07-23 07:17:40.289769	\N	nil authors test	\N	\N
963	955	\N	\N	\N	\N	Add Author Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-23 07:17:40.306254	2026-07-23 07:17:40.306254	2026-07-23 07:17:40.306254	\N	add author test	\N	\N
216	212	\N	\N	\N	\N	Add Author Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-12 12:18:03.06264	2026-07-23 07:17:48.878294	2026-07-12 12:18:03.06264	\N	add author test	\N	\N
964	956	\N	\N	\N	\N	Test Book Part 1	eng	Self-published	2023	Self-published	0	\N	\N	First test book created via API	manual	t	good	f	0	2026-07-24 13:13:24.566758	2026-07-24 13:13:24.566758	2026-07-24 13:13:24.566758	\N	test book part 1	\N	\N
965	957	\N	\N	\N	\N	Test Book Part 2	eng	Self-published	2024	Self-published	0	\N	\N	Second test book created via API	manual	t	good	f	0	2026-07-24 13:13:24.573495	2026-07-24 13:13:24.573495	2026-07-24 13:13:24.573495	\N	test book part 2	\N	\N
968	960	\N	\N	\N	\N	Original Title	eng	Self-published	2025	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-24 13:13:24.622676	2026-07-24 13:13:24.631225	2026-07-24 13:13:24.622676	\N	original title	\N	\N
969	961	\N	\N	\N	\N	Book With ISBN Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-24 13:13:24.646203	2026-07-24 13:13:24.646203	2026-07-24 13:13:24.646203	\N	book with isbn test	\N	\N
970	962	\N	\N	\N	\N	Book Without ISBN	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-24 13:13:24.872152	2026-07-24 13:13:24.872152	2026-07-24 13:13:24.872152	\N	book without isbn	\N	\N
972	964	\N	\N	\N	\N	Book Two	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-24 13:13:24.895048	2026-07-24 13:13:24.895048	2026-07-24 13:13:24.895048	\N	book two	\N	\N
954	946	\N	\N	\N	\N	Book One	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-23 07:17:39.675216	2026-07-24 13:13:24.8976	2026-07-23 07:17:39.675216	\N	book one	\N	\N
973	965	\N	\N	\N	\N	New Edition Title	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-24 13:13:24.908016	2026-07-24 13:13:24.914447	2026-07-24 13:13:24.908016	\N	new edition title	\N	\N
974	966	9780000000974	\N	\N	\N	Test Empty Strings	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-24 13:13:24.926932	2026-07-24 13:13:24.933052	2026-07-24 13:13:24.926932	\N	test empty strings	\N	\N
975	967	97800000009751	\N	\N	\N	Corrupted ISBN Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-24 13:13:24.943588	2026-07-24 13:13:24.950313	2026-07-24 13:13:24.943588	\N	corrupted isbn test	\N	\N
976	968	\N	\N	\N	\N	Remove Authors Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-24 13:13:25.455177	2026-07-24 13:13:25.455177	2026-07-24 13:13:25.455177	\N	remove authors test	\N	\N
946	938	978-5-17-088369-1	\N	\N	\N	Детский мир (сборник)	rus		2015	\N	\N	\N	\N	\N	imported	t	good	f	4	2026-07-23 07:07:55.587012	2026-07-24 18:58:42.400333	2026-07-23 07:07:55.587012	\N	детский мир (сборник)	\N	1
977	969	\N	\N	\N	\N	Remove Genres Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-24 13:13:25.473525	2026-07-24 13:13:25.473525	2026-07-24 13:13:25.473525	\N	remove genres test	\N	\N
978	970	\N	\N	\N	\N	Remove Tags Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-24 13:13:25.490529	2026-07-24 13:13:25.490529	2026-07-24 13:13:25.490529	\N	remove tags test	\N	\N
979	971	\N	\N	\N	\N	Nil Authors Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-24 13:13:25.509177	2026-07-24 13:13:25.509177	2026-07-24 13:13:25.509177	\N	nil authors test	\N	\N
980	972	\N	\N	\N	\N	Add Author Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-24 13:13:25.526854	2026-07-24 13:13:25.526854	2026-07-24 13:13:25.526854	\N	add author test	\N	\N
981	973	\N	\N	\N	\N	Test Book Part 1	eng	Self-published	2023	Self-published	0	\N	\N	First test book created via API	manual	t	good	f	0	2026-07-24 13:18:50.692058	2026-07-24 13:18:50.692058	2026-07-24 13:18:50.692058	\N	test book part 1	\N	\N
982	974	\N	\N	\N	\N	Test Book Part 2	eng	Self-published	2024	Self-published	0	\N	\N	Second test book created via API	manual	t	good	f	0	2026-07-24 13:18:50.698837	2026-07-24 13:18:50.698837	2026-07-24 13:18:50.698837	\N	test book part 2	\N	\N
983	975	\N	\N	\N	\N	Updated Book Title	eng	Self-published	2022	Self-published	0	\N	\N	Updated description	manual	t	good	f	0	2026-07-24 13:18:50.711429	2026-07-24 13:18:50.718905	2026-07-24 13:18:50.711429	\N	updated book title	\N	\N
985	977	\N	\N	\N	\N	Original Title	eng	Self-published	2025	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-24 13:18:50.751683	2026-07-24 13:18:50.760029	2026-07-24 13:18:50.751683	\N	original title	\N	\N
986	978	\N	\N	\N	\N	Book With ISBN Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-24 13:18:50.777676	2026-07-24 13:18:50.777676	2026-07-24 13:18:50.777676	\N	book with isbn test	\N	\N
987	979	\N	\N	\N	\N	Book Without ISBN	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-24 13:18:50.798002	2026-07-24 13:18:50.798002	2026-07-24 13:18:50.798002	\N	book without isbn	\N	\N
989	981	\N	\N	\N	\N	Book Two	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-24 13:18:50.822616	2026-07-24 13:18:50.822616	2026-07-24 13:18:50.822616	\N	book two	\N	\N
971	963	\N	\N	\N	\N	Book One	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-24 13:13:24.888625	2026-07-24 13:18:50.825143	2026-07-24 13:13:24.888625	\N	book one	\N	\N
990	982	\N	\N	\N	\N	New Edition Title	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-24 13:18:50.839801	2026-07-24 13:18:50.845712	2026-07-24 13:18:50.839801	\N	new edition title	\N	\N
991	983	9780000000991	\N	\N	\N	Test Empty Strings	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-24 13:18:50.859227	2026-07-24 13:18:50.865509	2026-07-24 13:18:50.859227	\N	test empty strings	\N	\N
992	984	97800000009921	\N	\N	\N	Corrupted ISBN Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-24 13:18:50.879886	2026-07-24 13:18:50.886285	2026-07-24 13:18:50.879886	\N	corrupted isbn test	\N	\N
993	985	\N	\N	\N	\N	Remove Authors Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-24 13:18:51.419246	2026-07-24 13:18:51.419246	2026-07-24 13:18:51.419246	\N	remove authors test	\N	\N
994	986	\N	\N	\N	\N	Remove Genres Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-24 13:18:51.441838	2026-07-24 13:18:51.441838	2026-07-24 13:18:51.441838	\N	remove genres test	\N	\N
995	987	\N	\N	\N	\N	Remove Tags Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-24 13:18:51.460151	2026-07-24 13:18:51.460151	2026-07-24 13:18:51.460151	\N	remove tags test	\N	\N
996	988	\N	\N	\N	\N	Nil Authors Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-24 13:18:51.476998	2026-07-24 13:18:51.476998	2026-07-24 13:18:51.476998	\N	nil authors test	\N	\N
997	989	\N	\N	\N	\N	Add Author Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-24 13:18:51.496996	2026-07-24 13:18:51.496996	2026-07-24 13:18:51.496996	\N	add author test	\N	\N
998	990	\N	\N	\N	\N	Test Book Part 1	eng	Self-published	2023	Self-published	0	\N	\N	First test book created via API	manual	t	good	f	0	2026-07-24 13:36:44.535594	2026-07-24 13:36:44.535594	2026-07-24 13:36:44.535594	\N	test book part 1	\N	\N
999	991	\N	\N	\N	\N	Test Book Part 2	eng	Self-published	2024	Self-published	0	\N	\N	Second test book created via API	manual	t	good	f	0	2026-07-24 13:36:44.541781	2026-07-24 13:36:44.541781	2026-07-24 13:36:44.541781	\N	test book part 2	\N	\N
1000	992	\N	\N	\N	\N	Updated Book Title	eng	Self-published	2022	Self-published	0	\N	\N	Updated description	manual	t	good	f	0	2026-07-24 13:36:44.552856	2026-07-24 13:36:44.558921	2026-07-24 13:36:44.552856	\N	updated book title	\N	\N
1002	994	\N	\N	\N	\N	Original Title	eng	Self-published	2025	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-24 13:36:44.586251	2026-07-24 13:36:44.595062	2026-07-24 13:36:44.586251	\N	original title	\N	\N
1003	995	\N	\N	\N	\N	Book With ISBN Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-24 13:36:44.610108	2026-07-24 13:36:44.610108	2026-07-24 13:36:44.610108	\N	book with isbn test	\N	\N
1004	996	\N	\N	\N	\N	Book Without ISBN	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-24 13:36:44.628097	2026-07-24 13:36:44.628097	2026-07-24 13:36:44.628097	\N	book without isbn	\N	\N
1006	998	\N	\N	\N	\N	Book Two	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-24 13:36:44.651616	2026-07-24 13:36:44.651616	2026-07-24 13:36:44.651616	\N	book two	\N	\N
988	980	\N	\N	\N	\N	Book One	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-24 13:18:50.815429	2026-07-24 13:36:44.653961	2026-07-24 13:18:50.815429	\N	book one	\N	\N
1007	999	\N	\N	\N	\N	New Edition Title	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-24 13:36:44.667209	2026-07-24 13:36:44.673384	2026-07-24 13:36:44.667209	\N	new edition title	\N	\N
1008	1000	9780000001008	\N	\N	\N	Test Empty Strings	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-24 13:36:44.685492	2026-07-24 13:36:44.692611	2026-07-24 13:36:44.685492	\N	test empty strings	\N	\N
1009	1001	97800000010091	\N	\N	\N	Corrupted ISBN Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-24 13:36:44.702462	2026-07-24 13:36:44.710744	2026-07-24 13:36:44.702462	\N	corrupted isbn test	\N	\N
1010	1002	\N	\N	\N	\N	Remove Authors Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-24 13:36:45.194849	2026-07-24 13:36:45.194849	2026-07-24 13:36:45.194849	\N	remove authors test	\N	\N
1011	1003	\N	\N	\N	\N	Remove Genres Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-24 13:36:45.212671	2026-07-24 13:36:45.212671	2026-07-24 13:36:45.212671	\N	remove genres test	\N	\N
1012	1004	\N	\N	\N	\N	Remove Tags Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-24 13:36:45.228532	2026-07-24 13:36:45.228532	2026-07-24 13:36:45.228532	\N	remove tags test	\N	\N
1013	1005	\N	\N	\N	\N	Nil Authors Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-24 13:36:45.24484	2026-07-24 13:36:45.24484	2026-07-24 13:36:45.24484	\N	nil authors test	\N	\N
1014	1006	\N	\N	\N	\N	Add Author Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-24 13:36:45.26059	2026-07-24 13:36:45.26059	2026-07-24 13:36:45.26059	\N	add author test	\N	\N
1015	1007	\N	\N	\N	\N	Test Book Part 1	eng	Self-published	2023	Self-published	0	\N	\N	First test book created via API	manual	t	good	f	0	2026-07-24 15:09:43.910989	2026-07-24 15:09:43.910989	2026-07-24 15:09:43.910989	\N	test book part 1	\N	\N
1016	1008	\N	\N	\N	\N	Test Book Part 2	eng	Self-published	2024	Self-published	0	\N	\N	Second test book created via API	manual	t	good	f	0	2026-07-24 15:09:43.917654	2026-07-24 15:09:43.917654	2026-07-24 15:09:43.917654	\N	test book part 2	\N	\N
1017	1009	\N	\N	\N	\N	Updated Book Title	eng	Self-published	2022	Self-published	0	\N	\N	Updated description	manual	t	good	f	0	2026-07-24 15:09:43.931523	2026-07-24 15:09:43.937495	2026-07-24 15:09:43.931523	\N	updated book title	\N	\N
1019	1011	\N	\N	\N	\N	Original Title	eng	Self-published	2025	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-24 15:09:43.965619	2026-07-24 15:09:43.975474	2026-07-24 15:09:43.965619	\N	original title	\N	\N
1020	1012	\N	\N	\N	\N	Book With ISBN Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-24 15:09:43.991359	2026-07-24 15:09:43.991359	2026-07-24 15:09:43.991359	\N	book with isbn test	\N	\N
1021	1013	\N	\N	\N	\N	Book Without ISBN	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-24 15:09:44.012588	2026-07-24 15:09:44.012588	2026-07-24 15:09:44.012588	\N	book without isbn	\N	\N
1023	1015	\N	\N	\N	\N	Book Two	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-24 15:09:44.038459	2026-07-24 15:09:44.038459	2026-07-24 15:09:44.038459	\N	book two	\N	\N
1022	1014	\N	\N	\N	\N	Book One	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-24 15:09:44.032559	2026-07-24 15:41:19.808871	2026-07-24 15:09:44.032559	\N	book one	\N	\N
1005	997	\N	\N	\N	\N	Book One	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-24 13:36:44.645743	2026-07-24 15:09:44.04079	2026-07-24 13:36:44.645743	\N	book one	\N	\N
1417	1409	97800000014171	\N	\N	\N	Corrupted ISBN Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-27 10:21:49.455955	2026-07-27 10:21:49.462826	2026-07-27 10:21:49.455955	\N	corrupted isbn test	\N	\N
1024	1016	\N	\N	\N	\N	New Edition Title	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-24 15:09:44.053328	2026-07-24 15:09:44.059376	2026-07-24 15:09:44.053328	\N	new edition title	\N	\N
1025	1017	9780000001025	\N	\N	\N	Test Empty Strings	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-24 15:09:44.072821	2026-07-24 15:09:44.078866	2026-07-24 15:09:44.072821	\N	test empty strings	\N	\N
1026	1018	97800000010261	\N	\N	\N	Corrupted ISBN Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-24 15:09:44.089424	2026-07-24 15:09:44.097059	2026-07-24 15:09:44.089424	\N	corrupted isbn test	\N	\N
1027	1019	\N	\N	\N	\N	Remove Authors Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-24 15:09:44.619403	2026-07-24 15:09:44.619403	2026-07-24 15:09:44.619403	\N	remove authors test	\N	\N
1028	1020	\N	\N	\N	\N	Remove Genres Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-24 15:09:44.638413	2026-07-24 15:09:44.638413	2026-07-24 15:09:44.638413	\N	remove genres test	\N	\N
1029	1021	\N	\N	\N	\N	Remove Tags Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-24 15:09:44.655964	2026-07-24 15:09:44.655964	2026-07-24 15:09:44.655964	\N	remove tags test	\N	\N
1030	1022	\N	\N	\N	\N	Nil Authors Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-24 15:09:44.672235	2026-07-24 15:09:44.672235	2026-07-24 15:09:44.672235	\N	nil authors test	\N	\N
1031	1023	\N	\N	\N	\N	Add Author Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-24 15:09:44.68828	2026-07-24 15:09:44.68828	2026-07-24 15:09:44.68828	\N	add author test	\N	\N
838	831	9785961473780	\N	\N	\N	Контекст жизни. Как научиться управлять привычками, которые управляют нами	rus		2021	\N	\N	\N	\N	\N	imported	t	good	f	1	2026-07-21 15:55:27.399981	2026-07-24 15:36:32.657472	2026-07-21 15:55:27.399981	\N	контекст жизни. как научиться управлять привычками, которые управляют нами	\N	1
1032	1024	\N	\N	\N	\N	Test Book Part 1	eng	Self-published	2023	Self-published	0	\N	\N	First test book created via API	manual	t	good	f	0	2026-07-24 15:41:19.686104	2026-07-24 15:41:19.686104	2026-07-24 15:41:19.686104	\N	test book part 1	\N	\N
1033	1025	\N	\N	\N	\N	Test Book Part 2	eng	Self-published	2024	Self-published	0	\N	\N	Second test book created via API	manual	t	good	f	0	2026-07-24 15:41:19.692653	2026-07-24 15:41:19.692653	2026-07-24 15:41:19.692653	\N	test book part 2	\N	\N
1034	1026	\N	\N	\N	\N	Updated Book Title	eng	Self-published	2022	Self-published	0	\N	\N	Updated description	manual	t	good	f	0	2026-07-24 15:41:19.707093	2026-07-24 15:41:19.713372	2026-07-24 15:41:19.707093	\N	updated book title	\N	\N
1036	1028	\N	\N	\N	\N	Original Title	eng	Self-published	2025	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-24 15:41:19.742577	2026-07-24 15:41:19.751211	2026-07-24 15:41:19.742577	\N	original title	\N	\N
1037	1029	\N	\N	\N	\N	Book With ISBN Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-24 15:41:19.767177	2026-07-24 15:41:19.767177	2026-07-24 15:41:19.767177	\N	book with isbn test	\N	\N
1038	1030	\N	\N	\N	\N	Book Without ISBN	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-24 15:41:19.78451	2026-07-24 15:41:19.78451	2026-07-24 15:41:19.78451	\N	book without isbn	\N	\N
1040	1032	\N	\N	\N	\N	Book Two	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-24 15:41:19.806294	2026-07-24 15:41:19.806294	2026-07-24 15:41:19.806294	\N	book two	\N	\N
1041	1033	\N	\N	\N	\N	New Edition Title	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-24 15:41:19.818826	2026-07-24 15:41:19.82528	2026-07-24 15:41:19.818826	\N	new edition title	\N	\N
1042	1034	9780000001042	\N	\N	\N	Test Empty Strings	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-24 15:41:19.836339	2026-07-24 15:41:19.843261	2026-07-24 15:41:19.836339	\N	test empty strings	\N	\N
1043	1035	97800000010431	\N	\N	\N	Corrupted ISBN Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-24 15:41:19.853324	2026-07-24 15:41:19.859691	2026-07-24 15:41:19.853324	\N	corrupted isbn test	\N	\N
1044	1036	\N	\N	\N	\N	Remove Authors Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-24 15:41:20.337568	2026-07-24 15:41:20.337568	2026-07-24 15:41:20.337568	\N	remove authors test	\N	\N
1045	1037	\N	\N	\N	\N	Remove Genres Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-24 15:41:20.354362	2026-07-24 15:41:20.354362	2026-07-24 15:41:20.354362	\N	remove genres test	\N	\N
1046	1038	\N	\N	\N	\N	Remove Tags Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-24 15:41:20.371216	2026-07-24 15:41:20.371216	2026-07-24 15:41:20.371216	\N	remove tags test	\N	\N
1047	1039	\N	\N	\N	\N	Nil Authors Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-24 15:41:20.387168	2026-07-24 15:41:20.387168	2026-07-24 15:41:20.387168	\N	nil authors test	\N	\N
1048	1040	\N	\N	\N	\N	Add Author Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-24 15:41:20.404612	2026-07-24 15:41:20.404612	2026-07-24 15:41:20.404612	\N	add author test	\N	\N
1049	1041	\N	\N	\N	\N	Test Book Part 1	eng	Self-published	2023	Self-published	0	\N	\N	First test book created via API	manual	t	good	f	0	2026-07-24 15:46:46.824363	2026-07-24 15:46:46.824363	2026-07-24 15:46:46.824363	\N	test book part 1	\N	\N
1050	1042	\N	\N	\N	\N	Test Book Part 2	eng	Self-published	2024	Self-published	0	\N	\N	Second test book created via API	manual	t	good	f	0	2026-07-24 15:46:46.830658	2026-07-24 15:46:46.830658	2026-07-24 15:46:46.830658	\N	test book part 2	\N	\N
1051	1043	\N	\N	\N	\N	Updated Book Title	eng	Self-published	2022	Self-published	0	\N	\N	Updated description	manual	t	good	f	0	2026-07-24 15:46:46.842188	2026-07-24 15:46:46.848502	2026-07-24 15:46:46.842188	\N	updated book title	\N	\N
1053	1045	\N	\N	\N	\N	Original Title	eng	Self-published	2025	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-24 15:46:46.878097	2026-07-24 15:46:46.886128	2026-07-24 15:46:46.878097	\N	original title	\N	\N
1054	1046	\N	\N	\N	\N	Book With ISBN Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-24 15:46:46.899645	2026-07-24 15:46:46.899645	2026-07-24 15:46:46.899645	\N	book with isbn test	\N	\N
1055	1047	\N	\N	\N	\N	Book Without ISBN	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-24 15:46:46.91668	2026-07-24 15:46:46.91668	2026-07-24 15:46:46.91668	\N	book without isbn	\N	\N
1057	1049	\N	\N	\N	\N	Book Two	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-24 15:46:46.93769	2026-07-24 15:46:46.93769	2026-07-24 15:46:46.93769	\N	book two	\N	\N
1039	1031	\N	\N	\N	\N	Book One	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-24 15:41:19.800165	2026-07-24 15:46:46.940842	2026-07-24 15:41:19.800165	\N	book one	\N	\N
1058	1050	\N	\N	\N	\N	New Edition Title	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-24 15:46:46.951962	2026-07-24 15:46:46.95775	2026-07-24 15:46:46.951962	\N	new edition title	\N	\N
1059	1051	9780000001059	\N	\N	\N	Test Empty Strings	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-24 15:46:46.969325	2026-07-24 15:46:46.975276	2026-07-24 15:46:46.969325	\N	test empty strings	\N	\N
1060	1052	97800000010601	\N	\N	\N	Corrupted ISBN Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-24 15:46:46.986683	2026-07-24 15:46:46.993415	2026-07-24 15:46:46.986683	\N	corrupted isbn test	\N	\N
1061	1053	\N	\N	\N	\N	Remove Authors Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-24 15:46:47.452354	2026-07-24 15:46:47.452354	2026-07-24 15:46:47.452354	\N	remove authors test	\N	\N
1062	1054	\N	\N	\N	\N	Remove Genres Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-24 15:46:47.470691	2026-07-24 15:46:47.470691	2026-07-24 15:46:47.470691	\N	remove genres test	\N	\N
1063	1055	\N	\N	\N	\N	Remove Tags Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-24 15:46:47.490046	2026-07-24 15:46:47.490046	2026-07-24 15:46:47.490046	\N	remove tags test	\N	\N
1064	1056	\N	\N	\N	\N	Nil Authors Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-24 15:46:47.509323	2026-07-24 15:46:47.509323	2026-07-24 15:46:47.509323	\N	nil authors test	\N	\N
1065	1057	\N	\N	\N	\N	Add Author Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-24 15:46:47.527781	2026-07-24 15:46:47.527781	2026-07-24 15:46:47.527781	\N	add author test	\N	\N
1066	1058	\N	\N	\N	\N	Test Book Part 1	eng	Self-published	2023	Self-published	0	\N	\N	First test book created via API	manual	t	good	f	0	2026-07-24 15:53:08.595811	2026-07-24 15:53:08.595811	2026-07-24 15:53:08.595811	\N	test book part 1	\N	\N
1067	1059	\N	\N	\N	\N	Test Book Part 2	eng	Self-published	2024	Self-published	0	\N	\N	Second test book created via API	manual	t	good	f	0	2026-07-24 15:53:08.602755	2026-07-24 15:53:08.602755	2026-07-24 15:53:08.602755	\N	test book part 2	\N	\N
1418	1410	\N	\N	\N	\N	Remove Authors Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-27 10:21:50.599706	2026-07-27 10:21:50.599706	2026-07-27 10:21:50.599706	\N	remove authors test	\N	\N
1068	1060	\N	\N	\N	\N	Updated Book Title	eng	Self-published	2022	Self-published	0	\N	\N	Updated description	manual	t	good	f	0	2026-07-24 15:53:08.615847	2026-07-24 15:53:08.621967	2026-07-24 15:53:08.615847	\N	updated book title	\N	\N
1070	1062	\N	\N	\N	\N	Original Title	eng	Self-published	2025	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-24 15:53:08.650741	2026-07-24 15:53:08.659759	2026-07-24 15:53:08.650741	\N	original title	\N	\N
1071	1063	\N	\N	\N	\N	Book With ISBN Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-24 15:53:08.673799	2026-07-24 15:53:08.673799	2026-07-24 15:53:08.673799	\N	book with isbn test	\N	\N
1072	1064	\N	\N	\N	\N	Book Without ISBN	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-24 15:53:08.692076	2026-07-24 15:53:08.692076	2026-07-24 15:53:08.692076	\N	book without isbn	\N	\N
1074	1066	\N	\N	\N	\N	Book Two	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-24 15:53:08.715887	2026-07-24 15:53:08.715887	2026-07-24 15:53:08.715887	\N	book two	\N	\N
1056	1048	\N	\N	\N	\N	Book One	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-24 15:46:46.931845	2026-07-24 15:53:08.718179	2026-07-24 15:46:46.931845	\N	book one	\N	\N
1075	1067	\N	\N	\N	\N	New Edition Title	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-24 15:53:08.727616	2026-07-24 15:53:08.735304	2026-07-24 15:53:08.727616	\N	new edition title	\N	\N
1076	1068	9780000001076	\N	\N	\N	Test Empty Strings	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-24 15:53:08.746351	2026-07-24 15:53:08.752446	2026-07-24 15:53:08.746351	\N	test empty strings	\N	\N
1077	1069	97800000010771	\N	\N	\N	Corrupted ISBN Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-24 15:53:08.762448	2026-07-24 15:53:08.768935	2026-07-24 15:53:08.762448	\N	corrupted isbn test	\N	\N
1078	1070	\N	\N	\N	\N	Remove Authors Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-24 15:53:09.246651	2026-07-24 15:53:09.246651	2026-07-24 15:53:09.246651	\N	remove authors test	\N	\N
1079	1071	\N	\N	\N	\N	Remove Genres Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-24 15:53:09.264586	2026-07-24 15:53:09.264586	2026-07-24 15:53:09.264586	\N	remove genres test	\N	\N
1080	1072	\N	\N	\N	\N	Remove Tags Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-24 15:53:09.281413	2026-07-24 15:53:09.281413	2026-07-24 15:53:09.281413	\N	remove tags test	\N	\N
1081	1073	\N	\N	\N	\N	Nil Authors Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-24 15:53:09.298026	2026-07-24 15:53:09.298026	2026-07-24 15:53:09.298026	\N	nil authors test	\N	\N
1082	1074	\N	\N	\N	\N	Add Author Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-24 15:53:09.31492	2026-07-24 15:53:09.31492	2026-07-24 15:53:09.31492	\N	add author test	\N	\N
1083	1075	\N	\N	\N	\N	Test Book Part 1	eng	Self-published	2023	Self-published	0	\N	\N	First test book created via API	manual	t	good	f	0	2026-07-24 15:54:21.473135	2026-07-24 15:54:21.473135	2026-07-24 15:54:21.473135	\N	test book part 1	\N	\N
1084	1076	\N	\N	\N	\N	Test Book Part 2	eng	Self-published	2024	Self-published	0	\N	\N	Second test book created via API	manual	t	good	f	0	2026-07-24 15:54:21.479475	2026-07-24 15:54:21.479475	2026-07-24 15:54:21.479475	\N	test book part 2	\N	\N
1085	1077	\N	\N	\N	\N	Updated Book Title	eng	Self-published	2022	Self-published	0	\N	\N	Updated description	manual	t	good	f	0	2026-07-24 15:54:21.492925	2026-07-24 15:54:21.499231	2026-07-24 15:54:21.492925	\N	updated book title	\N	\N
1087	1079	\N	\N	\N	\N	Original Title	eng	Self-published	2025	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-24 15:54:21.532338	2026-07-24 15:54:21.541757	2026-07-24 15:54:21.532338	\N	original title	\N	\N
1088	1080	\N	\N	\N	\N	Book With ISBN Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-24 15:54:21.555972	2026-07-24 15:54:21.555972	2026-07-24 15:54:21.555972	\N	book with isbn test	\N	\N
1089	1081	\N	\N	\N	\N	Book Without ISBN	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-24 15:54:21.573318	2026-07-24 15:54:21.573318	2026-07-24 15:54:21.573318	\N	book without isbn	\N	\N
1091	1083	\N	\N	\N	\N	Book Two	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-24 15:54:21.597821	2026-07-24 15:54:21.597821	2026-07-24 15:54:21.597821	\N	book two	\N	\N
1073	1065	\N	\N	\N	\N	Book One	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-24 15:53:08.709321	2026-07-24 15:54:21.600521	2026-07-24 15:53:08.709321	\N	book one	\N	\N
1092	1084	\N	\N	\N	\N	New Edition Title	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-24 15:54:21.61436	2026-07-24 15:54:21.620973	2026-07-24 15:54:21.61436	\N	new edition title	\N	\N
1093	1085	9780000001093	\N	\N	\N	Test Empty Strings	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-24 15:54:21.636086	2026-07-24 15:54:21.643843	2026-07-24 15:54:21.636086	\N	test empty strings	\N	\N
1094	1086	97800000010941	\N	\N	\N	Corrupted ISBN Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-24 15:54:21.656271	2026-07-24 15:54:21.662958	2026-07-24 15:54:21.656271	\N	corrupted isbn test	\N	\N
1095	1087	\N	\N	\N	\N	Remove Authors Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-24 15:54:22.157474	2026-07-24 15:54:22.157474	2026-07-24 15:54:22.157474	\N	remove authors test	\N	\N
1096	1088	\N	\N	\N	\N	Remove Genres Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-24 15:54:22.177833	2026-07-24 15:54:22.177833	2026-07-24 15:54:22.177833	\N	remove genres test	\N	\N
1097	1089	\N	\N	\N	\N	Remove Tags Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-24 15:54:22.195172	2026-07-24 15:54:22.195172	2026-07-24 15:54:22.195172	\N	remove tags test	\N	\N
1098	1090	\N	\N	\N	\N	Nil Authors Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-24 15:54:22.212479	2026-07-24 15:54:22.212479	2026-07-24 15:54:22.212479	\N	nil authors test	\N	\N
1099	1091	\N	\N	\N	\N	Add Author Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-24 15:54:22.228632	2026-07-24 15:54:22.228632	2026-07-24 15:54:22.228632	\N	add author test	\N	\N
1100	1092	\N	\N	\N	\N	Test Book Part 1	eng	Self-published	2023	Self-published	0	\N	\N	First test book created via API	manual	t	good	f	0	2026-07-24 19:06:25.640595	2026-07-24 19:06:25.640595	2026-07-24 19:06:25.640595	\N	test book part 1	\N	\N
1101	1093	\N	\N	\N	\N	Test Book Part 2	eng	Self-published	2024	Self-published	0	\N	\N	Second test book created via API	manual	t	good	f	0	2026-07-24 19:06:25.647304	2026-07-24 19:06:25.647304	2026-07-24 19:06:25.647304	\N	test book part 2	\N	\N
1102	1094	\N	\N	\N	\N	Updated Book Title	eng	Self-published	2022	Self-published	0	\N	\N	Updated description	manual	t	good	f	0	2026-07-24 19:06:25.660861	2026-07-24 19:06:25.66685	2026-07-24 19:06:25.660861	\N	updated book title	\N	\N
1104	1096	\N	\N	\N	\N	Original Title	eng	Self-published	2025	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-24 19:06:25.700338	2026-07-24 19:06:25.708627	2026-07-24 19:06:25.700338	\N	original title	\N	\N
1105	1097	\N	\N	\N	\N	Book With ISBN Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-24 19:06:25.723716	2026-07-24 19:06:25.723716	2026-07-24 19:06:25.723716	\N	book with isbn test	\N	\N
1106	1098	\N	\N	\N	\N	Book Without ISBN	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-24 19:06:25.746073	2026-07-24 19:06:25.746073	2026-07-24 19:06:25.746073	\N	book without isbn	\N	\N
1108	1100	\N	\N	\N	\N	Book Two	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-24 19:06:25.773304	2026-07-24 19:06:25.773304	2026-07-24 19:06:25.773304	\N	book two	\N	\N
1090	1082	\N	\N	\N	\N	Book One	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-24 15:54:21.591606	2026-07-24 19:06:25.775837	2026-07-24 15:54:21.591606	\N	book one	\N	\N
1109	1101	\N	\N	\N	\N	New Edition Title	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-24 19:06:25.788612	2026-07-24 19:06:25.794428	2026-07-24 19:06:25.788612	\N	new edition title	\N	\N
1110	1102	9780000001110	\N	\N	\N	Test Empty Strings	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-24 19:06:25.808211	2026-07-24 19:06:25.814054	2026-07-24 19:06:25.808211	\N	test empty strings	\N	\N
1107	1099	\N	\N	\N	\N	Book One	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-24 19:06:25.767342	2026-07-24 19:13:00.999914	2026-07-24 19:06:25.767342	\N	book one	\N	\N
1111	1103	97800000011111	\N	\N	\N	Corrupted ISBN Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-24 19:06:25.827394	2026-07-24 19:06:25.834591	2026-07-24 19:06:25.827394	\N	corrupted isbn test	\N	\N
1112	1104	\N	\N	\N	\N	Remove Authors Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-24 19:06:26.375313	2026-07-24 19:06:26.375313	2026-07-24 19:06:26.375313	\N	remove authors test	\N	\N
1113	1105	\N	\N	\N	\N	Remove Genres Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-24 19:06:26.393159	2026-07-24 19:06:26.393159	2026-07-24 19:06:26.393159	\N	remove genres test	\N	\N
1114	1106	\N	\N	\N	\N	Remove Tags Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-24 19:06:26.413001	2026-07-24 19:06:26.413001	2026-07-24 19:06:26.413001	\N	remove tags test	\N	\N
1115	1107	\N	\N	\N	\N	Nil Authors Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-24 19:06:26.430468	2026-07-24 19:06:26.430468	2026-07-24 19:06:26.430468	\N	nil authors test	\N	\N
1116	1108	\N	\N	\N	\N	Add Author Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-24 19:06:26.446678	2026-07-24 19:06:26.446678	2026-07-24 19:06:26.446678	\N	add author test	\N	\N
806	801	\N	\N	\N	\N	Тайная Доктрина: Синтез науки, религии и философии	eng		\N	\N	\N	\N	\N	\N	imported	t	good	f	0	2026-07-21 15:47:29.570287	2026-07-24 19:06:36.379373	2026-07-21 15:47:29.570287	\N	тайная доктрина: синтез науки, религии и философии	\N	1
1117	1109	\N	\N	\N	\N	Test Book Part 1	eng	Self-published	2023	Self-published	0	\N	\N	First test book created via API	manual	t	good	f	0	2026-07-24 19:13:00.870128	2026-07-24 19:13:00.870128	2026-07-24 19:13:00.870128	\N	test book part 1	\N	\N
1118	1110	\N	\N	\N	\N	Test Book Part 2	eng	Self-published	2024	Self-published	0	\N	\N	Second test book created via API	manual	t	good	f	0	2026-07-24 19:13:00.877423	2026-07-24 19:13:00.877423	2026-07-24 19:13:00.877423	\N	test book part 2	\N	\N
1419	1411	\N	\N	\N	\N	Remove Genres Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-27 10:21:50.619964	2026-07-27 10:21:50.619964	2026-07-27 10:21:50.619964	\N	remove genres test	\N	\N
1119	1111	\N	\N	\N	\N	Updated Book Title	eng	Self-published	2022	Self-published	0	\N	\N	Updated description	manual	t	good	f	0	2026-07-24 19:13:00.89039	2026-07-24 19:13:00.897218	2026-07-24 19:13:00.89039	\N	updated book title	\N	\N
1121	1113	\N	\N	\N	\N	Original Title	eng	Self-published	2025	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-24 19:13:00.927296	2026-07-24 19:13:00.935885	2026-07-24 19:13:00.927296	\N	original title	\N	\N
1122	1114	\N	\N	\N	\N	Book With ISBN Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-24 19:13:00.950338	2026-07-24 19:13:00.950338	2026-07-24 19:13:00.950338	\N	book with isbn test	\N	\N
1123	1115	\N	\N	\N	\N	Book Without ISBN	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-24 19:13:00.974985	2026-07-24 19:13:00.974985	2026-07-24 19:13:00.974985	\N	book without isbn	\N	\N
1125	1117	\N	\N	\N	\N	Book Two	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-24 19:13:00.996987	2026-07-24 19:13:00.996987	2026-07-24 19:13:00.996987	\N	book two	\N	\N
1126	1118	\N	\N	\N	\N	New Edition Title	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-24 19:13:01.009138	2026-07-24 19:13:01.015723	2026-07-24 19:13:01.009138	\N	new edition title	\N	\N
1127	1119	9780000001127	\N	\N	\N	Test Empty Strings	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-24 19:13:01.026899	2026-07-24 19:13:01.032903	2026-07-24 19:13:01.026899	\N	test empty strings	\N	\N
1128	1120	97800000011281	\N	\N	\N	Corrupted ISBN Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-24 19:13:01.045419	2026-07-24 19:13:01.052214	2026-07-24 19:13:01.045419	\N	corrupted isbn test	\N	\N
1129	1121	\N	\N	\N	\N	Remove Authors Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-24 19:13:01.568701	2026-07-24 19:13:01.568701	2026-07-24 19:13:01.568701	\N	remove authors test	\N	\N
1130	1122	\N	\N	\N	\N	Remove Genres Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-24 19:13:01.586883	2026-07-24 19:13:01.586883	2026-07-24 19:13:01.586883	\N	remove genres test	\N	\N
1131	1123	\N	\N	\N	\N	Remove Tags Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-24 19:13:01.609914	2026-07-24 19:13:01.609914	2026-07-24 19:13:01.609914	\N	remove tags test	\N	\N
1132	1124	\N	\N	\N	\N	Nil Authors Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-24 19:13:01.628404	2026-07-24 19:13:01.628404	2026-07-24 19:13:01.628404	\N	nil authors test	\N	\N
1133	1125	\N	\N	\N	\N	Add Author Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-24 19:13:01.649206	2026-07-24 19:13:01.649206	2026-07-24 19:13:01.649206	\N	add author test	\N	\N
1134	1126	\N	\N	\N	\N	Test Book Part 1	eng	Self-published	2023	Self-published	0	\N	\N	First test book created via API	manual	t	good	f	0	2026-07-24 19:47:46.415981	2026-07-24 19:47:46.415981	2026-07-24 19:47:46.415981	\N	test book part 1	\N	\N
1135	1127	\N	\N	\N	\N	Test Book Part 2	eng	Self-published	2024	Self-published	0	\N	\N	Second test book created via API	manual	t	good	f	0	2026-07-24 19:47:46.422245	2026-07-24 19:47:46.422245	2026-07-24 19:47:46.422245	\N	test book part 2	\N	\N
1136	1128	\N	\N	\N	\N	Updated Book Title	eng	Self-published	2022	Self-published	0	\N	\N	Updated description	manual	t	good	f	0	2026-07-24 19:47:46.433338	2026-07-24 19:47:46.439252	2026-07-24 19:47:46.433338	\N	updated book title	\N	\N
1138	1130	\N	\N	\N	\N	Original Title	eng	Self-published	2025	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-24 19:47:46.471255	2026-07-24 19:47:46.480157	2026-07-24 19:47:46.471255	\N	original title	\N	\N
1139	1131	\N	\N	\N	\N	Book With ISBN Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-24 19:47:46.493131	2026-07-24 19:47:46.493131	2026-07-24 19:47:46.493131	\N	book with isbn test	\N	\N
1140	1132	\N	\N	\N	\N	Book Without ISBN	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-24 19:47:46.510162	2026-07-24 19:47:46.510162	2026-07-24 19:47:46.510162	\N	book without isbn	\N	\N
1142	1134	\N	\N	\N	\N	Book Two	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-24 19:47:46.531306	2026-07-24 19:47:46.531306	2026-07-24 19:47:46.531306	\N	book two	\N	\N
1124	1116	\N	\N	\N	\N	Book One	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-24 19:13:00.990697	2026-07-24 19:47:46.534172	2026-07-24 19:13:00.990697	\N	book one	\N	\N
1143	1135	\N	\N	\N	\N	New Edition Title	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-24 19:47:46.544987	2026-07-24 19:47:46.550992	2026-07-24 19:47:46.544987	\N	new edition title	\N	\N
1144	1136	9780000001144	\N	\N	\N	Test Empty Strings	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-24 19:47:46.566817	2026-07-24 19:47:46.5733	2026-07-24 19:47:46.566817	\N	test empty strings	\N	\N
1145	1137	97800000011451	\N	\N	\N	Corrupted ISBN Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-24 19:47:46.582561	2026-07-24 19:47:46.589068	2026-07-24 19:47:46.582561	\N	corrupted isbn test	\N	\N
1146	1138	\N	\N	\N	\N	Remove Authors Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-24 19:47:47.086925	2026-07-24 19:47:47.086925	2026-07-24 19:47:47.086925	\N	remove authors test	\N	\N
1147	1139	\N	\N	\N	\N	Remove Genres Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-24 19:47:47.105484	2026-07-24 19:47:47.105484	2026-07-24 19:47:47.105484	\N	remove genres test	\N	\N
1148	1140	\N	\N	\N	\N	Remove Tags Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-24 19:47:47.131975	2026-07-24 19:47:47.131975	2026-07-24 19:47:47.131975	\N	remove tags test	\N	\N
1149	1141	\N	\N	\N	\N	Nil Authors Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-24 19:47:47.14891	2026-07-24 19:47:47.14891	2026-07-24 19:47:47.14891	\N	nil authors test	\N	\N
1150	1142	\N	\N	\N	\N	Add Author Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-24 19:47:47.163993	2026-07-24 19:47:47.163993	2026-07-24 19:47:47.163993	\N	add author test	\N	\N
1151	1143	\N	\N	\N	\N	Test Book Part 1	eng	Self-published	2023	Self-published	0	\N	\N	First test book created via API	manual	t	good	f	0	2026-07-24 19:48:03.634369	2026-07-24 19:48:03.634369	2026-07-24 19:48:03.634369	\N	test book part 1	\N	\N
1152	1144	\N	\N	\N	\N	Test Book Part 2	eng	Self-published	2024	Self-published	0	\N	\N	Second test book created via API	manual	t	good	f	0	2026-07-24 19:48:03.641471	2026-07-24 19:48:03.641471	2026-07-24 19:48:03.641471	\N	test book part 2	\N	\N
1153	1145	\N	\N	\N	\N	Updated Book Title	eng	Self-published	2022	Self-published	0	\N	\N	Updated description	manual	t	good	f	0	2026-07-24 19:48:03.654486	2026-07-24 19:48:03.660949	2026-07-24 19:48:03.654486	\N	updated book title	\N	\N
1141	1133	\N	\N	\N	\N	Book One	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-24 19:47:46.525577	2026-07-24 19:48:03.761921	2026-07-24 19:47:46.525577	\N	book one	\N	\N
1295	1287	\N	\N	\N	\N	Book Two	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-26 20:03:20.611036	2026-07-26 20:03:20.611036	2026-07-26 20:03:20.611036	\N	book two	\N	\N
1420	1412	\N	\N	\N	\N	Remove Tags Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-27 10:21:50.639665	2026-07-27 10:21:50.639665	2026-07-27 10:21:50.639665	\N	remove tags test	\N	\N
1155	1147	\N	\N	\N	\N	Original Title	eng	Self-published	2025	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-24 19:48:03.691668	2026-07-24 19:48:03.700764	2026-07-24 19:48:03.691668	\N	original title	\N	\N
1156	1148	\N	\N	\N	\N	Book With ISBN Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-24 19:48:03.715392	2026-07-24 19:48:03.715392	2026-07-24 19:48:03.715392	\N	book with isbn test	\N	\N
1157	1149	\N	\N	\N	\N	Book Without ISBN	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-24 19:48:03.735367	2026-07-24 19:48:03.735367	2026-07-24 19:48:03.735367	\N	book without isbn	\N	\N
1159	1151	\N	\N	\N	\N	Book Two	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-24 19:48:03.759338	2026-07-24 19:48:03.759338	2026-07-24 19:48:03.759338	\N	book two	\N	\N
1160	1152	\N	\N	\N	\N	New Edition Title	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-24 19:48:03.771414	2026-07-24 19:48:03.777692	2026-07-24 19:48:03.771414	\N	new edition title	\N	\N
1161	1153	9780000001161	\N	\N	\N	Test Empty Strings	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-24 19:48:03.790649	2026-07-24 19:48:03.799437	2026-07-24 19:48:03.790649	\N	test empty strings	\N	\N
1162	1154	97800000011621	\N	\N	\N	Corrupted ISBN Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-24 19:48:03.819755	2026-07-24 19:48:03.826885	2026-07-24 19:48:03.819755	\N	corrupted isbn test	\N	\N
1163	1155	\N	\N	\N	\N	Remove Authors Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-24 19:48:04.339376	2026-07-24 19:48:04.339376	2026-07-24 19:48:04.339376	\N	remove authors test	\N	\N
1164	1156	\N	\N	\N	\N	Remove Genres Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-24 19:48:04.356463	2026-07-24 19:48:04.356463	2026-07-24 19:48:04.356463	\N	remove genres test	\N	\N
1165	1157	\N	\N	\N	\N	Remove Tags Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-24 19:48:04.379497	2026-07-24 19:48:04.379497	2026-07-24 19:48:04.379497	\N	remove tags test	\N	\N
1166	1158	\N	\N	\N	\N	Nil Authors Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-24 19:48:04.39956	2026-07-24 19:48:04.39956	2026-07-24 19:48:04.39956	\N	nil authors test	\N	\N
1167	1159	\N	\N	\N	\N	Add Author Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-24 19:48:04.418486	2026-07-24 19:48:04.418486	2026-07-24 19:48:04.418486	\N	add author test	\N	\N
1168	1160	\N	\N	\N	\N	Test Book Part 1	eng	Self-published	2023	Self-published	0	\N	\N	First test book created via API	manual	t	good	f	0	2026-07-25 11:35:26.597435	2026-07-25 11:35:26.597435	2026-07-25 11:35:26.597435	\N	test book part 1	\N	\N
1169	1161	\N	\N	\N	\N	Test Book Part 2	eng	Self-published	2024	Self-published	0	\N	\N	Second test book created via API	manual	t	good	f	0	2026-07-25 11:35:26.618086	2026-07-25 11:35:26.618086	2026-07-25 11:35:26.618086	\N	test book part 2	\N	\N
1170	1162	\N	\N	\N	\N	Updated Book Title	eng	Self-published	2022	Self-published	0	\N	\N	Updated description	manual	t	good	f	0	2026-07-25 11:35:26.629088	2026-07-25 11:35:26.636269	2026-07-25 11:35:26.629088	\N	updated book title	\N	\N
1172	1164	\N	\N	\N	\N	Original Title	eng	Self-published	2025	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-25 11:35:26.670686	2026-07-25 11:35:26.67963	2026-07-25 11:35:26.670686	\N	original title	\N	\N
1173	1165	\N	\N	\N	\N	Book With ISBN Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-25 11:35:26.695544	2026-07-25 11:35:26.695544	2026-07-25 11:35:26.695544	\N	book with isbn test	\N	\N
1174	1166	\N	\N	\N	\N	Book Without ISBN	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-25 11:35:26.71329	2026-07-25 11:35:26.71329	2026-07-25 11:35:26.71329	\N	book without isbn	\N	\N
1176	1168	\N	\N	\N	\N	Book Two	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-25 11:35:26.737323	2026-07-25 11:35:26.737323	2026-07-25 11:35:26.737323	\N	book two	\N	\N
1158	1150	\N	\N	\N	\N	Book One	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-24 19:48:03.752975	2026-07-25 11:35:26.739771	2026-07-24 19:48:03.752975	\N	book one	\N	\N
1177	1169	\N	\N	\N	\N	New Edition Title	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-25 11:35:26.7503	2026-07-25 11:35:26.756199	2026-07-25 11:35:26.7503	\N	new edition title	\N	\N
1178	1170	9780000001178	\N	\N	\N	Test Empty Strings	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-25 11:35:26.767563	2026-07-25 11:35:26.773568	2026-07-25 11:35:26.767563	\N	test empty strings	\N	\N
1179	1171	97800000011791	\N	\N	\N	Corrupted ISBN Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-25 11:35:26.785185	2026-07-25 11:35:26.791858	2026-07-25 11:35:26.785185	\N	corrupted isbn test	\N	\N
1180	1172	\N	\N	\N	\N	Remove Authors Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-25 11:35:27.277602	2026-07-25 11:35:27.277602	2026-07-25 11:35:27.277602	\N	remove authors test	\N	\N
1181	1173	\N	\N	\N	\N	Remove Genres Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-25 11:35:27.295264	2026-07-25 11:35:27.295264	2026-07-25 11:35:27.295264	\N	remove genres test	\N	\N
1182	1174	\N	\N	\N	\N	Remove Tags Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-25 11:35:27.312914	2026-07-25 11:35:27.312914	2026-07-25 11:35:27.312914	\N	remove tags test	\N	\N
1183	1175	\N	\N	\N	\N	Nil Authors Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-25 11:35:27.328963	2026-07-25 11:35:27.328963	2026-07-25 11:35:27.328963	\N	nil authors test	\N	\N
1184	1176	\N	\N	\N	\N	Add Author Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-25 11:35:27.345399	2026-07-25 11:35:27.345399	2026-07-25 11:35:27.345399	\N	add author test	\N	\N
1185	1177	\N	\N	\N	\N	Test Book Part 1	eng	Self-published	2023	Self-published	0	\N	\N	First test book created via API	manual	t	good	f	0	2026-07-25 14:19:39.849752	2026-07-25 14:19:39.849752	2026-07-25 14:19:39.849752	\N	test book part 1	\N	\N
1186	1178	\N	\N	\N	\N	Test Book Part 2	eng	Self-published	2024	Self-published	0	\N	\N	Second test book created via API	manual	t	good	f	0	2026-07-25 14:19:39.858317	2026-07-25 14:19:39.858317	2026-07-25 14:19:39.858317	\N	test book part 2	\N	\N
1187	1179	\N	\N	\N	\N	Updated Book Title	eng	Self-published	2022	Self-published	0	\N	\N	Updated description	manual	t	good	f	0	2026-07-25 14:19:39.871557	2026-07-25 14:19:39.877374	2026-07-25 14:19:39.871557	\N	updated book title	\N	\N
1189	1181	\N	\N	\N	\N	Original Title	eng	Self-published	2025	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-25 14:19:39.911022	2026-07-25 14:19:39.919596	2026-07-25 14:19:39.911022	\N	original title	\N	\N
1190	1182	\N	\N	\N	\N	Book With ISBN Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-25 14:19:39.940497	2026-07-25 14:19:39.940497	2026-07-25 14:19:39.940497	\N	book with isbn test	\N	\N
1191	1183	\N	\N	\N	\N	Book Without ISBN	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-25 14:19:39.957652	2026-07-25 14:19:39.957652	2026-07-25 14:19:39.957652	\N	book without isbn	\N	\N
1193	1185	\N	\N	\N	\N	Book Two	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-25 14:19:39.980905	2026-07-25 14:19:39.980905	2026-07-25 14:19:39.980905	\N	book two	\N	\N
1175	1167	\N	\N	\N	\N	Book One	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-25 11:35:26.729228	2026-07-25 14:19:39.983743	2026-07-25 11:35:26.729228	\N	book one	\N	\N
1194	1186	\N	\N	\N	\N	New Edition Title	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-25 14:19:39.994795	2026-07-25 14:19:40.001043	2026-07-25 14:19:39.994795	\N	new edition title	\N	\N
1195	1187	9780000001195	\N	\N	\N	Test Empty Strings	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-25 14:19:40.014643	2026-07-25 14:19:40.020683	2026-07-25 14:19:40.014643	\N	test empty strings	\N	\N
1196	1188	97800000011961	\N	\N	\N	Corrupted ISBN Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-25 14:19:40.03149	2026-07-25 14:19:40.037841	2026-07-25 14:19:40.03149	\N	corrupted isbn test	\N	\N
1197	1189	\N	\N	\N	\N	Remove Authors Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-25 14:19:40.517551	2026-07-25 14:19:40.517551	2026-07-25 14:19:40.517551	\N	remove authors test	\N	\N
1198	1190	\N	\N	\N	\N	Remove Genres Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-25 14:19:40.536549	2026-07-25 14:19:40.536549	2026-07-25 14:19:40.536549	\N	remove genres test	\N	\N
1199	1191	\N	\N	\N	\N	Remove Tags Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-25 14:19:40.556586	2026-07-25 14:19:40.556586	2026-07-25 14:19:40.556586	\N	remove tags test	\N	\N
1200	1192	\N	\N	\N	\N	Nil Authors Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-25 14:19:40.574113	2026-07-25 14:19:40.574113	2026-07-25 14:19:40.574113	\N	nil authors test	\N	\N
1201	1193	\N	\N	\N	\N	Add Author Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-25 14:19:40.59136	2026-07-25 14:19:40.59136	2026-07-25 14:19:40.59136	\N	add author test	\N	\N
1202	1194	\N	\N	\N	\N	Test Book Part 1	eng	Self-published	2023	Self-published	0	\N	\N	First test book created via API	manual	t	good	f	0	2026-07-25 15:04:15.330683	2026-07-25 15:04:15.330683	2026-07-25 15:04:15.330683	\N	test book part 1	\N	\N
1203	1195	\N	\N	\N	\N	Test Book Part 2	eng	Self-published	2024	Self-published	0	\N	\N	Second test book created via API	manual	t	good	f	0	2026-07-25 15:04:15.33773	2026-07-25 15:04:15.33773	2026-07-25 15:04:15.33773	\N	test book part 2	\N	\N
1421	1413	\N	\N	\N	\N	Nil Authors Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-27 10:21:50.656015	2026-07-27 10:21:50.656015	2026-07-27 10:21:50.656015	\N	nil authors test	\N	\N
1204	1196	\N	\N	\N	\N	Updated Book Title	eng	Self-published	2022	Self-published	0	\N	\N	Updated description	manual	t	good	f	0	2026-07-25 15:04:15.349899	2026-07-25 15:04:15.356658	2026-07-25 15:04:15.349899	\N	updated book title	\N	\N
1206	1198	\N	\N	\N	\N	Original Title	eng	Self-published	2025	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-25 15:04:15.388347	2026-07-25 15:04:15.396362	2026-07-25 15:04:15.388347	\N	original title	\N	\N
1207	1199	\N	\N	\N	\N	Book With ISBN Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-25 15:04:15.410197	2026-07-25 15:04:15.410197	2026-07-25 15:04:15.410197	\N	book with isbn test	\N	\N
1208	1200	\N	\N	\N	\N	Book Without ISBN	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-25 15:04:15.429545	2026-07-25 15:04:15.429545	2026-07-25 15:04:15.429545	\N	book without isbn	\N	\N
1210	1202	\N	\N	\N	\N	Book Two	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-25 15:04:15.455178	2026-07-25 15:04:15.455178	2026-07-25 15:04:15.455178	\N	book two	\N	\N
1192	1184	\N	\N	\N	\N	Book One	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-25 14:19:39.974833	2026-07-25 15:04:15.457892	2026-07-25 14:19:39.974833	\N	book one	\N	\N
1211	1203	\N	\N	\N	\N	New Edition Title	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-25 15:04:15.467931	2026-07-25 15:04:15.473763	2026-07-25 15:04:15.467931	\N	new edition title	\N	\N
1212	1204	9780000001212	\N	\N	\N	Test Empty Strings	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-25 15:04:15.485409	2026-07-25 15:04:15.491455	2026-07-25 15:04:15.485409	\N	test empty strings	\N	\N
1213	1205	97800000012131	\N	\N	\N	Corrupted ISBN Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-25 15:04:15.501876	2026-07-25 15:04:15.508584	2026-07-25 15:04:15.501876	\N	corrupted isbn test	\N	\N
1214	1206	\N	\N	\N	\N	Remove Authors Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-25 15:04:16.008767	2026-07-25 15:04:16.008767	2026-07-25 15:04:16.008767	\N	remove authors test	\N	\N
1215	1207	\N	\N	\N	\N	Remove Genres Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-25 15:04:16.027255	2026-07-25 15:04:16.027255	2026-07-25 15:04:16.027255	\N	remove genres test	\N	\N
1216	1208	\N	\N	\N	\N	Remove Tags Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-25 15:04:16.044203	2026-07-25 15:04:16.044203	2026-07-25 15:04:16.044203	\N	remove tags test	\N	\N
1217	1209	\N	\N	\N	\N	Nil Authors Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-25 15:04:16.060537	2026-07-25 15:04:16.060537	2026-07-25 15:04:16.060537	\N	nil authors test	\N	\N
1218	1210	\N	\N	\N	\N	Add Author Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-25 15:04:16.078449	2026-07-25 15:04:16.078449	2026-07-25 15:04:16.078449	\N	add author test	\N	\N
1219	1211	\N	\N	\N	\N	Test Book Part 1	eng	Self-published	2023	Self-published	0	\N	\N	First test book created via API	manual	t	good	f	0	2026-07-25 15:29:01.853788	2026-07-25 15:29:01.853788	2026-07-25 15:29:01.853788	\N	test book part 1	\N	\N
1220	1212	\N	\N	\N	\N	Test Book Part 2	eng	Self-published	2024	Self-published	0	\N	\N	Second test book created via API	manual	t	good	f	0	2026-07-25 15:29:01.86069	2026-07-25 15:29:01.86069	2026-07-25 15:29:01.86069	\N	test book part 2	\N	\N
1221	1213	\N	\N	\N	\N	Updated Book Title	eng	Self-published	2022	Self-published	0	\N	\N	Updated description	manual	t	good	f	0	2026-07-25 15:29:01.872967	2026-07-25 15:29:01.879385	2026-07-25 15:29:01.872967	\N	updated book title	\N	\N
1223	1215	\N	\N	\N	\N	Original Title	eng	Self-published	2025	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-25 15:29:01.909197	2026-07-25 15:29:01.918897	2026-07-25 15:29:01.909197	\N	original title	\N	\N
1224	1216	\N	\N	\N	\N	Book With ISBN Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-25 15:29:01.932952	2026-07-25 15:29:01.932952	2026-07-25 15:29:01.932952	\N	book with isbn test	\N	\N
1225	1217	\N	\N	\N	\N	Book Without ISBN	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-25 15:29:01.949971	2026-07-25 15:29:01.949971	2026-07-25 15:29:01.949971	\N	book without isbn	\N	\N
1227	1219	\N	\N	\N	\N	Book Two	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-25 15:29:01.982144	2026-07-25 15:29:01.982144	2026-07-25 15:29:01.982144	\N	book two	\N	\N
1209	1201	\N	\N	\N	\N	Book One	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-25 15:04:15.449545	2026-07-25 15:29:01.984631	2026-07-25 15:04:15.449545	\N	book one	\N	\N
1228	1220	\N	\N	\N	\N	New Edition Title	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-25 15:29:01.994111	2026-07-25 15:29:01.999917	2026-07-25 15:29:01.994111	\N	new edition title	\N	\N
1229	1221	9780000001229	\N	\N	\N	Test Empty Strings	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-25 15:29:02.012812	2026-07-25 15:29:02.01853	2026-07-25 15:29:02.012812	\N	test empty strings	\N	\N
1230	1222	97800000012301	\N	\N	\N	Corrupted ISBN Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-25 15:29:02.029	2026-07-25 15:29:02.035352	2026-07-25 15:29:02.029	\N	corrupted isbn test	\N	\N
1231	1223	\N	\N	\N	\N	Remove Authors Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-25 15:29:02.519947	2026-07-25 15:29:02.519947	2026-07-25 15:29:02.519947	\N	remove authors test	\N	\N
1232	1224	\N	\N	\N	\N	Remove Genres Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-25 15:29:02.538807	2026-07-25 15:29:02.538807	2026-07-25 15:29:02.538807	\N	remove genres test	\N	\N
1233	1225	\N	\N	\N	\N	Remove Tags Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-25 15:29:02.557888	2026-07-25 15:29:02.557888	2026-07-25 15:29:02.557888	\N	remove tags test	\N	\N
1234	1226	\N	\N	\N	\N	Nil Authors Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-25 15:29:02.574292	2026-07-25 15:29:02.574292	2026-07-25 15:29:02.574292	\N	nil authors test	\N	\N
1235	1227	\N	\N	\N	\N	Add Author Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-25 15:29:02.589087	2026-07-25 15:29:02.589087	2026-07-25 15:29:02.589087	\N	add author test	\N	\N
1236	1228	\N	\N	\N	\N	Test Book Part 1	eng	Self-published	2023	Self-published	0	\N	\N	First test book created via API	manual	t	good	f	0	2026-07-25 15:47:11.629139	2026-07-25 15:47:11.629139	2026-07-25 15:47:11.629139	\N	test book part 1	\N	\N
1237	1229	\N	\N	\N	\N	Test Book Part 2	eng	Self-published	2024	Self-published	0	\N	\N	Second test book created via API	manual	t	good	f	0	2026-07-25 15:47:11.635446	2026-07-25 15:47:11.635446	2026-07-25 15:47:11.635446	\N	test book part 2	\N	\N
1238	1230	\N	\N	\N	\N	Updated Book Title	eng	Self-published	2022	Self-published	0	\N	\N	Updated description	manual	t	good	f	0	2026-07-25 15:47:11.65204	2026-07-25 15:47:11.658144	2026-07-25 15:47:11.65204	\N	updated book title	\N	\N
1240	1232	\N	\N	\N	\N	Original Title	eng	Self-published	2025	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-25 15:47:11.692465	2026-07-25 15:47:11.7013	2026-07-25 15:47:11.692465	\N	original title	\N	\N
1241	1233	\N	\N	\N	\N	Book With ISBN Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-25 15:47:11.716246	2026-07-25 15:47:11.716246	2026-07-25 15:47:11.716246	\N	book with isbn test	\N	\N
1242	1234	\N	\N	\N	\N	Book Without ISBN	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-25 15:47:11.735915	2026-07-25 15:47:11.735915	2026-07-25 15:47:11.735915	\N	book without isbn	\N	\N
1244	1236	\N	\N	\N	\N	Book Two	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-25 15:47:11.760246	2026-07-25 15:47:11.760246	2026-07-25 15:47:11.760246	\N	book two	\N	\N
1226	1218	\N	\N	\N	\N	Book One	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-25 15:29:01.975503	2026-07-25 15:47:11.762671	2026-07-25 15:29:01.975503	\N	book one	\N	\N
1245	1237	\N	\N	\N	\N	New Edition Title	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-25 15:47:11.781467	2026-07-25 15:47:11.788054	2026-07-25 15:47:11.781467	\N	new edition title	\N	\N
1246	1238	9780000001246	\N	\N	\N	Test Empty Strings	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-25 15:47:11.801097	2026-07-25 15:47:11.807398	2026-07-25 15:47:11.801097	\N	test empty strings	\N	\N
1243	1235	\N	\N	\N	\N	Book One	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-25 15:47:11.754176	2026-07-25 15:54:34.85689	2026-07-25 15:47:11.754176	\N	book one	\N	\N
1247	1239	97800000012471	\N	\N	\N	Corrupted ISBN Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-25 15:47:11.820388	2026-07-25 15:47:11.826891	2026-07-25 15:47:11.820388	\N	corrupted isbn test	\N	\N
1248	1240	\N	\N	\N	\N	Remove Authors Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-25 15:47:12.333721	2026-07-25 15:47:12.333721	2026-07-25 15:47:12.333721	\N	remove authors test	\N	\N
1249	1241	\N	\N	\N	\N	Remove Genres Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-25 15:47:12.360108	2026-07-25 15:47:12.360108	2026-07-25 15:47:12.360108	\N	remove genres test	\N	\N
1250	1242	\N	\N	\N	\N	Remove Tags Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-25 15:47:12.393971	2026-07-25 15:47:12.393971	2026-07-25 15:47:12.393971	\N	remove tags test	\N	\N
1251	1243	\N	\N	\N	\N	Nil Authors Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-25 15:47:12.414296	2026-07-25 15:47:12.414296	2026-07-25 15:47:12.414296	\N	nil authors test	\N	\N
1252	1244	\N	\N	\N	\N	Add Author Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-25 15:47:12.442687	2026-07-25 15:47:12.442687	2026-07-25 15:47:12.442687	\N	add author test	\N	\N
1253	1245	\N	\N	\N	\N	Test Book Part 1	eng	Self-published	2023	Self-published	0	\N	\N	First test book created via API	manual	t	good	f	0	2026-07-25 15:54:34.743911	2026-07-25 15:54:34.743911	2026-07-25 15:54:34.743911	\N	test book part 1	\N	\N
1254	1246	\N	\N	\N	\N	Test Book Part 2	eng	Self-published	2024	Self-published	0	\N	\N	Second test book created via API	manual	t	good	f	0	2026-07-25 15:54:34.749968	2026-07-25 15:54:34.749968	2026-07-25 15:54:34.749968	\N	test book part 2	\N	\N
1422	1414	\N	\N	\N	\N	Add Author Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-27 10:21:50.673931	2026-07-27 10:21:50.673931	2026-07-27 10:21:50.673931	\N	add author test	\N	\N
1255	1247	\N	\N	\N	\N	Updated Book Title	eng	Self-published	2022	Self-published	0	\N	\N	Updated description	manual	t	good	f	0	2026-07-25 15:54:34.760406	2026-07-25 15:54:34.766309	2026-07-25 15:54:34.760406	\N	updated book title	\N	\N
1257	1249	\N	\N	\N	\N	Original Title	eng	Self-published	2025	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-25 15:54:34.793323	2026-07-25 15:54:34.801991	2026-07-25 15:54:34.793323	\N	original title	\N	\N
1258	1250	\N	\N	\N	\N	Book With ISBN Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-25 15:54:34.816539	2026-07-25 15:54:34.816539	2026-07-25 15:54:34.816539	\N	book with isbn test	\N	\N
1259	1251	\N	\N	\N	\N	Book Without ISBN	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-25 15:54:34.834187	2026-07-25 15:54:34.834187	2026-07-25 15:54:34.834187	\N	book without isbn	\N	\N
1261	1253	\N	\N	\N	\N	Book Two	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-25 15:54:34.854424	2026-07-25 15:54:34.854424	2026-07-25 15:54:34.854424	\N	book two	\N	\N
1262	1254	\N	\N	\N	\N	New Edition Title	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-25 15:54:34.867163	2026-07-25 15:54:34.873312	2026-07-25 15:54:34.867163	\N	new edition title	\N	\N
1263	1255	9780000001263	\N	\N	\N	Test Empty Strings	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-25 15:54:34.887096	2026-07-25 15:54:34.893436	2026-07-25 15:54:34.887096	\N	test empty strings	\N	\N
1264	1256	97800000012641	\N	\N	\N	Corrupted ISBN Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-25 15:54:34.904952	2026-07-25 15:54:34.911598	2026-07-25 15:54:34.904952	\N	corrupted isbn test	\N	\N
1265	1257	\N	\N	\N	\N	Remove Authors Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-25 15:54:35.392974	2026-07-25 15:54:35.392974	2026-07-25 15:54:35.392974	\N	remove authors test	\N	\N
1266	1258	\N	\N	\N	\N	Remove Genres Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-25 15:54:35.411116	2026-07-25 15:54:35.411116	2026-07-25 15:54:35.411116	\N	remove genres test	\N	\N
1267	1259	\N	\N	\N	\N	Remove Tags Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-25 15:54:35.42709	2026-07-25 15:54:35.42709	2026-07-25 15:54:35.42709	\N	remove tags test	\N	\N
1268	1260	\N	\N	\N	\N	Nil Authors Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-25 15:54:35.444918	2026-07-25 15:54:35.444918	2026-07-25 15:54:35.444918	\N	nil authors test	\N	\N
1269	1261	\N	\N	\N	\N	Add Author Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-25 15:54:35.463791	2026-07-25 15:54:35.463791	2026-07-25 15:54:35.463791	\N	add author test	\N	\N
1270	1262	\N	\N	\N	\N	Test Book Part 1	eng	Self-published	2023	Self-published	0	\N	\N	First test book created via API	manual	t	good	f	0	2026-07-26 19:34:10.629768	2026-07-26 19:34:10.629768	2026-07-26 19:34:10.629768	\N	test book part 1	\N	\N
1271	1263	\N	\N	\N	\N	Test Book Part 2	eng	Self-published	2024	Self-published	0	\N	\N	Second test book created via API	manual	t	good	f	0	2026-07-26 19:34:10.64703	2026-07-26 19:34:10.64703	2026-07-26 19:34:10.64703	\N	test book part 2	\N	\N
1272	1264	\N	\N	\N	\N	Updated Book Title	eng	Self-published	2022	Self-published	0	\N	\N	Updated description	manual	t	good	f	0	2026-07-26 19:34:10.665086	2026-07-26 19:34:10.672009	2026-07-26 19:34:10.665086	\N	updated book title	\N	\N
1274	1266	\N	\N	\N	\N	Original Title	eng	Self-published	2025	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-26 19:34:10.711772	2026-07-26 19:34:10.721852	2026-07-26 19:34:10.711772	\N	original title	\N	\N
1275	1267	\N	\N	\N	\N	Book With ISBN Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-26 19:34:10.738556	2026-07-26 19:34:10.738556	2026-07-26 19:34:10.738556	\N	book with isbn test	\N	\N
1276	1268	\N	\N	\N	\N	Book Without ISBN	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-26 19:34:10.760565	2026-07-26 19:34:10.760565	2026-07-26 19:34:10.760565	\N	book without isbn	\N	\N
1278	1270	\N	\N	\N	\N	Book Two	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-26 19:34:10.784682	2026-07-26 19:34:10.784682	2026-07-26 19:34:10.784682	\N	book two	\N	\N
1260	1252	\N	\N	\N	\N	Book One	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-25 15:54:34.848658	2026-07-26 19:34:10.787334	2026-07-25 15:54:34.848658	\N	book one	\N	\N
1279	1271	\N	\N	\N	\N	New Edition Title	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-26 19:34:10.799533	2026-07-26 19:34:10.806495	2026-07-26 19:34:10.799533	\N	new edition title	\N	\N
1280	1272	9780000001280	\N	\N	\N	Test Empty Strings	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-26 19:34:10.818841	2026-07-26 19:34:10.825701	2026-07-26 19:34:10.818841	\N	test empty strings	\N	\N
1281	1273	97800000012811	\N	\N	\N	Corrupted ISBN Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-26 19:34:10.84004	2026-07-26 19:34:10.847213	2026-07-26 19:34:10.84004	\N	corrupted isbn test	\N	\N
1282	1274	\N	\N	\N	\N	Remove Authors Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-26 19:34:11.40391	2026-07-26 19:34:11.40391	2026-07-26 19:34:11.40391	\N	remove authors test	\N	\N
1283	1275	\N	\N	\N	\N	Remove Genres Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-26 19:34:11.42334	2026-07-26 19:34:11.42334	2026-07-26 19:34:11.42334	\N	remove genres test	\N	\N
1284	1276	\N	\N	\N	\N	Remove Tags Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-26 19:34:11.44492	2026-07-26 19:34:11.44492	2026-07-26 19:34:11.44492	\N	remove tags test	\N	\N
1285	1277	\N	\N	\N	\N	Nil Authors Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-26 19:34:11.46355	2026-07-26 19:34:11.46355	2026-07-26 19:34:11.46355	\N	nil authors test	\N	\N
1286	1278	\N	\N	\N	\N	Add Author Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-26 19:34:11.480723	2026-07-26 19:34:11.480723	2026-07-26 19:34:11.480723	\N	add author test	\N	\N
1287	1279	\N	\N	\N	\N	Test Book Part 1	eng	Self-published	2023	Self-published	0	\N	\N	First test book created via API	manual	t	good	f	0	2026-07-26 20:03:20.490721	2026-07-26 20:03:20.490721	2026-07-26 20:03:20.490721	\N	test book part 1	\N	\N
1288	1280	\N	\N	\N	\N	Test Book Part 2	eng	Self-published	2024	Self-published	0	\N	\N	Second test book created via API	manual	t	good	f	0	2026-07-26 20:03:20.497074	2026-07-26 20:03:20.497074	2026-07-26 20:03:20.497074	\N	test book part 2	\N	\N
1289	1281	\N	\N	\N	\N	Updated Book Title	eng	Self-published	2022	Self-published	0	\N	\N	Updated description	manual	t	good	f	0	2026-07-26 20:03:20.508698	2026-07-26 20:03:20.514734	2026-07-26 20:03:20.508698	\N	updated book title	\N	\N
1291	1283	\N	\N	\N	\N	Original Title	eng	Self-published	2025	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-26 20:03:20.545579	2026-07-26 20:03:20.554686	2026-07-26 20:03:20.545579	\N	original title	\N	\N
1292	1284	\N	\N	\N	\N	Book With ISBN Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-26 20:03:20.571758	2026-07-26 20:03:20.571758	2026-07-26 20:03:20.571758	\N	book with isbn test	\N	\N
1293	1285	\N	\N	\N	\N	Book Without ISBN	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-26 20:03:20.588725	2026-07-26 20:03:20.588725	2026-07-26 20:03:20.588725	\N	book without isbn	\N	\N
1277	1269	\N	\N	\N	\N	Book One	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-26 19:34:10.778269	2026-07-26 20:03:20.613461	2026-07-26 19:34:10.778269	\N	book one	\N	\N
1296	1288	\N	\N	\N	\N	New Edition Title	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-26 20:03:20.623674	2026-07-26 20:03:20.629857	2026-07-26 20:03:20.623674	\N	new edition title	\N	\N
1297	1289	9780000001297	\N	\N	\N	Test Empty Strings	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-26 20:03:20.640799	2026-07-26 20:03:20.646968	2026-07-26 20:03:20.640799	\N	test empty strings	\N	\N
1298	1290	97800000012981	\N	\N	\N	Corrupted ISBN Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-26 20:03:20.656595	2026-07-26 20:03:20.663475	2026-07-26 20:03:20.656595	\N	corrupted isbn test	\N	\N
1299	1291	\N	\N	\N	\N	Remove Authors Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-26 20:03:21.218299	2026-07-26 20:03:21.218299	2026-07-26 20:03:21.218299	\N	remove authors test	\N	\N
1300	1292	\N	\N	\N	\N	Remove Genres Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-26 20:03:21.235127	2026-07-26 20:03:21.235127	2026-07-26 20:03:21.235127	\N	remove genres test	\N	\N
1301	1293	\N	\N	\N	\N	Remove Tags Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-26 20:03:21.255066	2026-07-26 20:03:21.255066	2026-07-26 20:03:21.255066	\N	remove tags test	\N	\N
1302	1294	\N	\N	\N	\N	Nil Authors Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-26 20:03:21.271093	2026-07-26 20:03:21.271093	2026-07-26 20:03:21.271093	\N	nil authors test	\N	\N
1303	1295	\N	\N	\N	\N	Add Author Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-26 20:03:21.286791	2026-07-26 20:03:21.286791	2026-07-26 20:03:21.286791	\N	add author test	\N	\N
1304	1296	\N	\N	\N	\N	Test Book Part 1	eng	Self-published	2023	Self-published	0	\N	\N	First test book created via API	manual	t	good	f	0	2026-07-26 20:50:17.488358	2026-07-26 20:50:17.488358	2026-07-26 20:50:17.488358	\N	test book part 1	\N	\N
1305	1297	\N	\N	\N	\N	Test Book Part 2	eng	Self-published	2024	Self-published	0	\N	\N	Second test book created via API	manual	t	good	f	0	2026-07-26 20:50:17.497292	2026-07-26 20:50:17.497292	2026-07-26 20:50:17.497292	\N	test book part 2	\N	\N
1306	1298	\N	\N	\N	\N	Updated Book Title	eng	Self-published	2022	Self-published	0	\N	\N	Updated description	manual	t	good	f	0	2026-07-26 20:50:17.50793	2026-07-26 20:50:17.514947	2026-07-26 20:50:17.50793	\N	updated book title	\N	\N
1308	1300	\N	\N	\N	\N	Original Title	eng	Self-published	2025	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-26 20:50:17.546186	2026-07-26 20:50:17.5558	2026-07-26 20:50:17.546186	\N	original title	\N	\N
1309	1301	\N	\N	\N	\N	Book With ISBN Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-26 20:50:17.573667	2026-07-26 20:50:17.573667	2026-07-26 20:50:17.573667	\N	book with isbn test	\N	\N
1310	1302	\N	\N	\N	\N	Book Without ISBN	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-26 20:50:17.594869	2026-07-26 20:50:17.594869	2026-07-26 20:50:17.594869	\N	book without isbn	\N	\N
1312	1304	\N	\N	\N	\N	Book Two	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-26 20:50:17.617712	2026-07-26 20:50:17.617712	2026-07-26 20:50:17.617712	\N	book two	\N	\N
1294	1286	\N	\N	\N	\N	Book One	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-26 20:03:20.604812	2026-07-26 20:50:17.620249	2026-07-26 20:03:20.604812	\N	book one	\N	\N
1313	1305	\N	\N	\N	\N	New Edition Title	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-26 20:50:17.630028	2026-07-26 20:50:17.636022	2026-07-26 20:50:17.630028	\N	new edition title	\N	\N
1314	1306	9780000001314	\N	\N	\N	Test Empty Strings	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-26 20:50:17.647073	2026-07-26 20:50:17.653368	2026-07-26 20:50:17.647073	\N	test empty strings	\N	\N
1315	1307	97800000013151	\N	\N	\N	Corrupted ISBN Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-26 20:50:17.666102	2026-07-26 20:50:17.674138	2026-07-26 20:50:17.666102	\N	corrupted isbn test	\N	\N
1316	1308	\N	\N	\N	\N	Remove Authors Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-26 20:50:18.183151	2026-07-26 20:50:18.183151	2026-07-26 20:50:18.183151	\N	remove authors test	\N	\N
1317	1309	\N	\N	\N	\N	Remove Genres Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-26 20:50:18.204483	2026-07-26 20:50:18.204483	2026-07-26 20:50:18.204483	\N	remove genres test	\N	\N
1318	1310	\N	\N	\N	\N	Remove Tags Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-26 20:50:18.223243	2026-07-26 20:50:18.223243	2026-07-26 20:50:18.223243	\N	remove tags test	\N	\N
1319	1311	\N	\N	\N	\N	Nil Authors Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-26 20:50:18.239006	2026-07-26 20:50:18.239006	2026-07-26 20:50:18.239006	\N	nil authors test	\N	\N
1320	1312	\N	\N	\N	\N	Add Author Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-26 20:50:18.25431	2026-07-26 20:50:18.25431	2026-07-26 20:50:18.25431	\N	add author test	\N	\N
1321	1313	\N	\N	\N	\N	Test Book Part 1	eng	Self-published	2023	Self-published	0	\N	\N	First test book created via API	manual	t	good	f	0	2026-07-26 21:03:21.375001	2026-07-26 21:03:21.375001	2026-07-26 21:03:21.375001	\N	test book part 1	\N	\N
1322	1314	\N	\N	\N	\N	Test Book Part 2	eng	Self-published	2024	Self-published	0	\N	\N	Second test book created via API	manual	t	good	f	0	2026-07-26 21:03:21.381478	2026-07-26 21:03:21.381478	2026-07-26 21:03:21.381478	\N	test book part 2	\N	\N
1323	1315	\N	\N	\N	\N	Updated Book Title	eng	Self-published	2022	Self-published	0	\N	\N	Updated description	manual	t	good	f	0	2026-07-26 21:03:21.393274	2026-07-26 21:03:21.399804	2026-07-26 21:03:21.393274	\N	updated book title	\N	\N
1325	1317	\N	\N	\N	\N	Original Title	eng	Self-published	2025	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-26 21:03:21.427703	2026-07-26 21:03:21.43596	2026-07-26 21:03:21.427703	\N	original title	\N	\N
1326	1318	\N	\N	\N	\N	Book With ISBN Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-26 21:03:21.451382	2026-07-26 21:03:21.451382	2026-07-26 21:03:21.451382	\N	book with isbn test	\N	\N
1327	1319	\N	\N	\N	\N	Book Without ISBN	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-26 21:03:21.469133	2026-07-26 21:03:21.469133	2026-07-26 21:03:21.469133	\N	book without isbn	\N	\N
1329	1321	\N	\N	\N	\N	Book Two	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-26 21:03:21.491856	2026-07-26 21:03:21.491856	2026-07-26 21:03:21.491856	\N	book two	\N	\N
1311	1303	\N	\N	\N	\N	Book One	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-26 20:50:17.611735	2026-07-26 21:03:21.494087	2026-07-26 20:50:17.611735	\N	book one	\N	\N
1330	1322	\N	\N	\N	\N	New Edition Title	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-26 21:03:21.504041	2026-07-26 21:03:21.510955	2026-07-26 21:03:21.504041	\N	new edition title	\N	\N
1331	1323	9780000001331	\N	\N	\N	Test Empty Strings	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-26 21:03:21.52175	2026-07-26 21:03:21.528641	2026-07-26 21:03:21.52175	\N	test empty strings	\N	\N
1332	1324	97800000013321	\N	\N	\N	Corrupted ISBN Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-26 21:03:21.538295	2026-07-26 21:03:21.545364	2026-07-26 21:03:21.538295	\N	corrupted isbn test	\N	\N
1333	1325	\N	\N	\N	\N	Remove Authors Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-26 21:03:22.038481	2026-07-26 21:03:22.038481	2026-07-26 21:03:22.038481	\N	remove authors test	\N	\N
1334	1326	\N	\N	\N	\N	Remove Genres Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-26 21:03:22.055907	2026-07-26 21:03:22.055907	2026-07-26 21:03:22.055907	\N	remove genres test	\N	\N
1335	1327	\N	\N	\N	\N	Remove Tags Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-26 21:03:22.072202	2026-07-26 21:03:22.072202	2026-07-26 21:03:22.072202	\N	remove tags test	\N	\N
1336	1328	\N	\N	\N	\N	Nil Authors Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-26 21:03:22.091257	2026-07-26 21:03:22.091257	2026-07-26 21:03:22.091257	\N	nil authors test	\N	\N
1337	1329	\N	\N	\N	\N	Add Author Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-26 21:03:22.107418	2026-07-26 21:03:22.107418	2026-07-26 21:03:22.107418	\N	add author test	\N	\N
1338	1330	\N	\N	\N	\N	Test Book Part 1	eng	Self-published	2023	Self-published	0	\N	\N	First test book created via API	manual	t	good	f	0	2026-07-27 07:08:39.906875	2026-07-27 07:08:39.906875	2026-07-27 07:08:39.906875	\N	test book part 1	\N	\N
1339	1331	\N	\N	\N	\N	Test Book Part 2	eng	Self-published	2024	Self-published	0	\N	\N	Second test book created via API	manual	t	good	f	0	2026-07-27 07:08:39.913707	2026-07-27 07:08:39.913707	2026-07-27 07:08:39.913707	\N	test book part 2	\N	\N
1328	1320	\N	\N	\N	\N	Book One	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-26 21:03:21.486114	2026-07-27 07:08:40.02981	2026-07-26 21:03:21.486114	\N	book one	\N	\N
1340	1332	\N	\N	\N	\N	Updated Book Title	eng	Self-published	2022	Self-published	0	\N	\N	Updated description	manual	t	good	f	0	2026-07-27 07:08:39.924879	2026-07-27 07:08:39.930505	2026-07-27 07:08:39.924879	\N	updated book title	\N	\N
1342	1334	\N	\N	\N	\N	Original Title	eng	Self-published	2025	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-27 07:08:39.963112	2026-07-27 07:08:39.972116	2026-07-27 07:08:39.963112	\N	original title	\N	\N
1343	1335	\N	\N	\N	\N	Book With ISBN Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-27 07:08:39.98599	2026-07-27 07:08:39.98599	2026-07-27 07:08:39.98599	\N	book with isbn test	\N	\N
1344	1336	\N	\N	\N	\N	Book Without ISBN	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-27 07:08:40.004416	2026-07-27 07:08:40.004416	2026-07-27 07:08:40.004416	\N	book without isbn	\N	\N
1346	1338	\N	\N	\N	\N	Book Two	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-27 07:08:40.027235	2026-07-27 07:08:40.027235	2026-07-27 07:08:40.027235	\N	book two	\N	\N
1347	1339	\N	\N	\N	\N	New Edition Title	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-27 07:08:40.041454	2026-07-27 07:08:40.048812	2026-07-27 07:08:40.041454	\N	new edition title	\N	\N
1348	1340	9780000001348	\N	\N	\N	Test Empty Strings	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-27 07:08:40.061855	2026-07-27 07:08:40.068452	2026-07-27 07:08:40.061855	\N	test empty strings	\N	\N
1349	1341	97800000013491	\N	\N	\N	Corrupted ISBN Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-27 07:08:40.079275	2026-07-27 07:08:40.086074	2026-07-27 07:08:40.079275	\N	corrupted isbn test	\N	\N
1350	1342	\N	\N	\N	\N	Remove Authors Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-27 07:08:41.210459	2026-07-27 07:08:41.210459	2026-07-27 07:08:41.210459	\N	remove authors test	\N	\N
1351	1343	\N	\N	\N	\N	Remove Genres Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-27 07:08:41.226276	2026-07-27 07:08:41.226276	2026-07-27 07:08:41.226276	\N	remove genres test	\N	\N
1352	1344	\N	\N	\N	\N	Remove Tags Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-27 07:08:41.242867	2026-07-27 07:08:41.242867	2026-07-27 07:08:41.242867	\N	remove tags test	\N	\N
1353	1345	\N	\N	\N	\N	Nil Authors Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-27 07:08:41.25961	2026-07-27 07:08:41.25961	2026-07-27 07:08:41.25961	\N	nil authors test	\N	\N
1354	1346	\N	\N	\N	\N	Add Author Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-27 07:08:41.277766	2026-07-27 07:08:41.277766	2026-07-27 07:08:41.277766	\N	add author test	\N	\N
1355	1347	\N	\N	\N	\N	Test Book Part 1	eng	Self-published	2023	Self-published	0	\N	\N	First test book created via API	manual	t	good	f	0	2026-07-27 07:14:51.216994	2026-07-27 07:14:51.216994	2026-07-27 07:14:51.216994	\N	test book part 1	\N	\N
1356	1348	\N	\N	\N	\N	Test Book Part 2	eng	Self-published	2024	Self-published	0	\N	\N	Second test book created via API	manual	t	good	f	0	2026-07-27 07:14:51.224802	2026-07-27 07:14:51.224802	2026-07-27 07:14:51.224802	\N	test book part 2	\N	\N
1357	1349	\N	\N	\N	\N	Updated Book Title	eng	Self-published	2022	Self-published	0	\N	\N	Updated description	manual	t	good	f	0	2026-07-27 07:14:51.240208	2026-07-27 07:14:51.246065	2026-07-27 07:14:51.240208	\N	updated book title	\N	\N
1359	1351	\N	\N	\N	\N	Original Title	eng	Self-published	2025	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-27 07:14:51.2788	2026-07-27 07:14:51.288111	2026-07-27 07:14:51.2788	\N	original title	\N	\N
1360	1352	\N	\N	\N	\N	Book With ISBN Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-27 07:14:51.302791	2026-07-27 07:14:51.302791	2026-07-27 07:14:51.302791	\N	book with isbn test	\N	\N
1361	1353	\N	\N	\N	\N	Book Without ISBN	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-27 07:14:51.32056	2026-07-27 07:14:51.32056	2026-07-27 07:14:51.32056	\N	book without isbn	\N	\N
1363	1355	\N	\N	\N	\N	Book Two	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-27 07:14:51.342091	2026-07-27 07:14:51.342091	2026-07-27 07:14:51.342091	\N	book two	\N	\N
1345	1337	\N	\N	\N	\N	Book One	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-27 07:08:40.020814	2026-07-27 07:14:51.344653	2026-07-27 07:08:40.020814	\N	book one	\N	\N
1364	1356	\N	\N	\N	\N	New Edition Title	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-27 07:14:51.360027	2026-07-27 07:14:51.366025	2026-07-27 07:14:51.360027	\N	new edition title	\N	\N
1365	1357	9780000001365	\N	\N	\N	Test Empty Strings	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-27 07:14:51.377868	2026-07-27 07:14:51.384759	2026-07-27 07:14:51.377868	\N	test empty strings	\N	\N
1366	1358	97800000013661	\N	\N	\N	Corrupted ISBN Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-27 07:14:51.394203	2026-07-27 07:14:51.401453	2026-07-27 07:14:51.394203	\N	corrupted isbn test	\N	\N
1367	1359	\N	\N	\N	\N	Remove Authors Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-27 07:14:52.509815	2026-07-27 07:14:52.509815	2026-07-27 07:14:52.509815	\N	remove authors test	\N	\N
1368	1360	\N	\N	\N	\N	Remove Genres Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-27 07:14:52.52767	2026-07-27 07:14:52.52767	2026-07-27 07:14:52.52767	\N	remove genres test	\N	\N
1369	1361	\N	\N	\N	\N	Remove Tags Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-27 07:14:52.544735	2026-07-27 07:14:52.544735	2026-07-27 07:14:52.544735	\N	remove tags test	\N	\N
1370	1362	\N	\N	\N	\N	Nil Authors Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-27 07:14:52.56123	2026-07-27 07:14:52.56123	2026-07-27 07:14:52.56123	\N	nil authors test	\N	\N
1371	1363	\N	\N	\N	\N	Add Author Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-27 07:14:52.579477	2026-07-27 07:14:52.579477	2026-07-27 07:14:52.579477	\N	add author test	\N	\N
1372	1364	\N	\N	\N	\N	Test Book Part 1	eng	Self-published	2023	Self-published	0	\N	\N	First test book created via API	manual	t	good	f	0	2026-07-27 10:12:33.25546	2026-07-27 10:12:33.25546	2026-07-27 10:12:33.25546	\N	test book part 1	\N	\N
1373	1365	\N	\N	\N	\N	Test Book Part 2	eng	Self-published	2024	Self-published	0	\N	\N	Second test book created via API	manual	t	good	f	0	2026-07-27 10:12:33.263035	2026-07-27 10:12:33.263035	2026-07-27 10:12:33.263035	\N	test book part 2	\N	\N
1374	1366	\N	\N	\N	\N	Updated Book Title	eng	Self-published	2022	Self-published	0	\N	\N	Updated description	manual	t	good	f	0	2026-07-27 10:12:33.274156	2026-07-27 10:12:33.279953	2026-07-27 10:12:33.274156	\N	updated book title	\N	\N
1376	1368	\N	\N	\N	\N	Original Title	eng	Self-published	2025	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-27 10:12:33.310288	2026-07-27 10:12:33.32014	2026-07-27 10:12:33.310288	\N	original title	\N	\N
1377	1369	\N	\N	\N	\N	Book With ISBN Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-27 10:12:33.334671	2026-07-27 10:12:33.334671	2026-07-27 10:12:33.334671	\N	book with isbn test	\N	\N
1378	1370	\N	\N	\N	\N	Book Without ISBN	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-27 10:12:33.352159	2026-07-27 10:12:33.352159	2026-07-27 10:12:33.352159	\N	book without isbn	\N	\N
1380	1372	\N	\N	\N	\N	Book Two	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-27 10:12:33.376496	2026-07-27 10:12:33.376496	2026-07-27 10:12:33.376496	\N	book two	\N	\N
1362	1354	\N	\N	\N	\N	Book One	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-27 07:14:51.336307	2026-07-27 10:12:33.37929	2026-07-27 07:14:51.336307	\N	book one	\N	\N
1381	1373	\N	\N	\N	\N	New Edition Title	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-27 10:12:33.389334	2026-07-27 10:12:33.395801	2026-07-27 10:12:33.389334	\N	new edition title	\N	\N
1382	1374	9780000001382	\N	\N	\N	Test Empty Strings	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-27 10:12:33.406755	2026-07-27 10:12:33.41268	2026-07-27 10:12:33.406755	\N	test empty strings	\N	\N
1383	1375	97800000013831	\N	\N	\N	Corrupted ISBN Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-27 10:12:33.422122	2026-07-27 10:12:33.429231	2026-07-27 10:12:33.422122	\N	corrupted isbn test	\N	\N
1384	1376	\N	\N	\N	\N	Remove Authors Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-27 10:12:34.585056	2026-07-27 10:12:34.585056	2026-07-27 10:12:34.585056	\N	remove authors test	\N	\N
1385	1377	\N	\N	\N	\N	Remove Genres Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-27 10:12:34.604261	2026-07-27 10:12:34.604261	2026-07-27 10:12:34.604261	\N	remove genres test	\N	\N
1386	1378	\N	\N	\N	\N	Remove Tags Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-27 10:12:34.620689	2026-07-27 10:12:34.620689	2026-07-27 10:12:34.620689	\N	remove tags test	\N	\N
1379	1371	\N	\N	\N	\N	Book One	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-27 10:12:33.369804	2026-07-27 10:17:32.068643	2026-07-27 10:12:33.369804	\N	book one	\N	\N
1387	1379	\N	\N	\N	\N	Nil Authors Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-27 10:12:34.638973	2026-07-27 10:12:34.638973	2026-07-27 10:12:34.638973	\N	nil authors test	\N	\N
1388	1380	\N	\N	\N	\N	Add Author Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-27 10:12:34.655902	2026-07-27 10:12:34.655902	2026-07-27 10:12:34.655902	\N	add author test	\N	\N
1389	1381	\N	\N	\N	\N	Test Book Part 1	eng	Self-published	2023	Self-published	0	\N	\N	First test book created via API	manual	t	good	f	0	2026-07-27 10:17:31.94385	2026-07-27 10:17:31.94385	2026-07-27 10:17:31.94385	\N	test book part 1	\N	\N
1390	1382	\N	\N	\N	\N	Test Book Part 2	eng	Self-published	2024	Self-published	0	\N	\N	Second test book created via API	manual	t	good	f	0	2026-07-27 10:17:31.951307	2026-07-27 10:17:31.951307	2026-07-27 10:17:31.951307	\N	test book part 2	\N	\N
1396	1388	\N	\N	\N	\N	Book One	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-27 10:17:32.058987	2026-07-27 10:21:49.410501	2026-07-27 10:17:32.058987	\N	book one	\N	\N
1391	1383	\N	\N	\N	\N	Updated Book Title	eng	Self-published	2022	Self-published	0	\N	\N	Updated description	manual	t	good	f	0	2026-07-27 10:17:31.962338	2026-07-27 10:17:31.968461	2026-07-27 10:17:31.962338	\N	updated book title	\N	\N
1393	1385	\N	\N	\N	\N	Original Title	eng	Self-published	2025	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-27 10:17:32.000471	2026-07-27 10:17:32.009302	2026-07-27 10:17:32.000471	\N	original title	\N	\N
1394	1386	\N	\N	\N	\N	Book With ISBN Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-27 10:17:32.023428	2026-07-27 10:17:32.023428	2026-07-27 10:17:32.023428	\N	book with isbn test	\N	\N
1395	1387	\N	\N	\N	\N	Book Without ISBN	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-27 10:17:32.040193	2026-07-27 10:17:32.040193	2026-07-27 10:17:32.040193	\N	book without isbn	\N	\N
1397	1389	\N	\N	\N	\N	Book Two	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-27 10:17:32.065933	2026-07-27 10:17:32.065933	2026-07-27 10:17:32.065933	\N	book two	\N	\N
1398	1390	\N	\N	\N	\N	New Edition Title	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-27 10:17:32.081033	2026-07-27 10:17:32.087534	2026-07-27 10:17:32.081033	\N	new edition title	\N	\N
1399	1391	9780000001399	\N	\N	\N	Test Empty Strings	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-27 10:17:32.100048	2026-07-27 10:17:32.106342	2026-07-27 10:17:32.100048	\N	test empty strings	\N	\N
1400	1392	97800000014001	\N	\N	\N	Corrupted ISBN Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-27 10:17:32.116928	2026-07-27 10:17:32.124388	2026-07-27 10:17:32.116928	\N	corrupted isbn test	\N	\N
1401	1393	\N	\N	\N	\N	Remove Authors Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-27 10:17:33.289592	2026-07-27 10:17:33.289592	2026-07-27 10:17:33.289592	\N	remove authors test	\N	\N
1402	1394	\N	\N	\N	\N	Remove Genres Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-27 10:17:33.310168	2026-07-27 10:17:33.310168	2026-07-27 10:17:33.310168	\N	remove genres test	\N	\N
1403	1395	\N	\N	\N	\N	Remove Tags Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-27 10:17:33.327002	2026-07-27 10:17:33.327002	2026-07-27 10:17:33.327002	\N	remove tags test	\N	\N
1404	1396	\N	\N	\N	\N	Nil Authors Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-27 10:17:33.342821	2026-07-27 10:17:33.342821	2026-07-27 10:17:33.342821	\N	nil authors test	\N	\N
1405	1397	\N	\N	\N	\N	Add Author Test	eng	Self-published	0	Self-published	0	\N	\N		manual	t	good	f	0	2026-07-27 10:17:33.358699	2026-07-27 10:17:33.358699	2026-07-27 10:17:33.358699	\N	add author test	\N	\N
\.


--
-- Data for Name: formats; Type: TABLE DATA; Schema: public; Owner: -
--

COPY public.formats (id, name, extension, mime_type, category, is_reflowable, is_editable) FROM stdin;
1	FB2	fb2	application/x-fictionbook+xml	ebook	t	f
2	FB2.ZIP	fb2.zip	application/x-zip-compressed	ebook	t	f
3	EPUB	epub	application/epub+zip	ebook	t	f
4	MOBI	mobi	application/x-mobipocket-ebook	ebook	t	f
5	AZW3	azw3	application/vnd.amazon.ebook	ebook	t	f
6	PDF	pdf	application/pdf	document	f	f
7	DJVU	djvu	image/vnd.djvu	scanned	f	f
8	DOC	doc	application/msword	document	t	t
9	DOCX	docx	application/vnd.openxmlformats-officedocument.wordprocessingml.document	document	t	t
10	RTF	rtf	application/rtf	document	t	t
11	TXT	txt	text/plain	ebook	t	t
12	HTML	html	text/html	ebook	t	t
13	CBZ	cbz	application/x-cbz	comics	f	f
14	CBR	cbr	application/x-cbr	comics	f	f
\.


--
-- Data for Name: genres; Type: TABLE DATA; Schema: public; Owner: -
--

COPY public.genres (id, name, parent_id, description) FROM stdin;
1	management	\N	\N
2	prose_contemporary	\N	\N
3	adv_indian	\N	\N
6	sf_social	\N	\N
7	nonf_publicism	\N	\N
10	adv_history	\N	\N
16	foreign_adventure	\N	\N
19	literature_19	\N	\N
23	foreign_prose	\N	\N
26	adv_maritime	\N	\N
27	literature_20	\N	\N
28	prose_rus_classic	\N	\N
29	child_education	\N	\N
30	sci_business	\N	\N
31	nonf_biography	\N	\N
32	economics	\N	\N
33	foreign_edu	\N	\N
34	sci_philosophy	\N	\N
35	religion_esoterics	\N	\N
38	sci_politics	\N	\N
40	religion_self	\N	\N
50	russian_contemporary	\N	\N
54	sci_psychology	\N	\N
55	prose_classic	\N	\N
59	thriller	\N	\N
63	popular_business	\N	\N
65	sf	\N	\N
66	sf_humor	\N	\N
72	religion_rel	\N	\N
76	antique_east	\N	\N
80	nonf_criticism	\N	\N
115	prose_counter	\N	\N
176	antique_ant	\N	\N
181	religion	\N	\N
186	DetectiveGenreTest	\N	\N
\.


--
-- Data for Name: import_sessions; Type: TABLE DATA; Schema: public; Owner: -
--

COPY public.import_sessions (id, source_type, source_path, started_at, finished_at, total_processed, new_works, new_editions, new_files, duplicates_found, errors, status) FROM stdin;
\.


--
-- Data for Name: languages; Type: TABLE DATA; Schema: public; Owner: -
--

COPY public.languages (code, name, native_name) FROM stdin;
rus	Russian	Русский
eng	English	English
deu	German	Deutsch
fra	French	Français
spa	Spanish	Español
ita	Italian	Italiano
jpn	Japanese	日本語
chi	Chinese	中文
ara	Arabic	العربية
por	Portuguese	Português
ukr	Ukrainian	Українська
bel	Belarusian	Беларуская
\.


--
-- Data for Name: persons; Type: TABLE DATA; Schema: public; Owner: -
--

COPY public.persons (id, first_name, middle_name, last_name, pseudonym, birth_date, death_date, biography, photo_url, created_at, lower_fio) FROM stdin;
916	Георгий	\N	Щедровицкий	\N	\N	\N	\N	\N	2026-07-21 15:44:10.227431	щедровицкий георгий
2	Виктор	\N	Пелевин	\N	\N	\N	\N	\N	2026-07-10 16:54:00.432714	пелевин виктор
918	Нил Дональд	\N	Уолш	\N	\N	\N	\N	\N	2026-07-21 15:44:10.306207	уолш нил дональд
919	Майн	\N	Рид	\N	\N	\N	\N	\N	2026-07-21 15:44:10.335852	рид майн
922	Владислав	\N	Петров	\N	\N	\N	\N	\N	2026-07-21 15:44:10.52771	петров владислав
923	Виктор	\N	Чуманов	\N	\N	\N	\N	\N	2026-07-21 15:44:10.52771	чуманов виктор
925	Анатолий	\N	Гланц	\N	\N	\N	\N	\N	2026-07-21 15:44:10.52771	гланц анатолий
926	Дмитрий	\N	Семеновский	\N	\N	\N	\N	\N	2026-07-21 15:44:10.52771	семеновский дмитрий
927	Виктор	\N	Каменский	\N	\N	\N	\N	\N	2026-07-21 15:44:10.52771	каменский виктор
928	Николай	\N	Глазков	\N	\N	\N	\N	\N	2026-07-21 15:44:10.52771	глазков николай
929	Даниил	\N	Клугер	\N	\N	\N	\N	\N	2026-07-21 15:44:10.52771	клугер даниил
930	Михаил	\N	Айзенберг	\N	\N	\N	\N	\N	2026-07-21 15:44:10.52771	айзенберг михаил
931	Виталий	\N	Бабенко	\N	\N	\N	\N	\N	2026-07-21 15:44:10.52771	бабенко виталий
932	Филиппо	\N	Маринетти	\N	\N	\N	\N	\N	2026-07-21 15:44:10.52771	маринетти филиппо
933	Игорь	\N	Бестужев-Лада	\N	\N	\N	\N	\N	2026-07-21 15:44:10.52771	бестужев-лада игорь
934	Норман	\N	Спинрад	\N	\N	\N	\N	\N	2026-07-21 15:44:10.52771	спинрад норман
935	Владимир	\N	Жуков	\N	\N	\N	\N	\N	2026-07-21 15:44:10.52771	жуков владимир
936	Евгений	\N	Лукин	\N	\N	\N	\N	\N	2026-07-21 15:44:10.52771	лукин евгений
937	Евгений	\N	Маевский	\N	\N	\N	\N	\N	2026-07-21 15:44:10.52771	маевский евгений
938	Михаил	\N	Бескин	\N	\N	\N	\N	\N	2026-07-21 15:44:10.52771	бескин михаил
939	Робер	\N	Деснос	\N	\N	\N	\N	\N	2026-07-21 15:44:10.52771	деснос робер
940	Юрий	\N	Левитанский	\N	\N	\N	\N	\N	2026-07-21 15:44:10.52771	левитанский юрий
941	Дмитрий	\N	Быков	\N	\N	\N	\N	\N	2026-07-21 15:44:10.52771	быков дмитрий
942	Василий	\N	Князев	\N	\N	\N	\N	\N	2026-07-21 15:44:10.52771	князев василий
943	Иосиф	\N	Сталин	\N	\N	\N	\N	\N	2026-07-21 15:44:10.52771	сталин иосиф
944	Вячеслав	\N	Рыбаков	\N	\N	\N	\N	\N	2026-07-21 15:44:10.52771	рыбаков вячеслав
945	Михаил	\N	Успенский	\N	\N	\N	\N	\N	2026-07-21 15:44:10.52771	успенский михаил
946	Людмила	\N	Петрушевская	\N	\N	\N	\N	\N	2026-07-21 15:44:10.593699	петрушевская людмила
947	Ольга	\N	Трифонова	\N	\N	\N	\N	\N	2026-07-21 15:44:10.593699	трифонова ольга
948	Александр	\N	Кабаков	\N	\N	\N	\N	\N	2026-07-21 15:44:10.593699	кабаков александр
949	Анна	\N	Матвеева	\N	\N	\N	\N	\N	2026-07-21 15:44:10.593699	матвеева анна
950	Михаил	\N	Веллер	\N	\N	\N	\N	\N	2026-07-21 15:44:10.593699	веллер михаил
951	Людмила	\N	Улицкая	\N	\N	\N	\N	\N	2026-07-21 15:44:10.593699	улицкая людмила
952	Василий	\N	Аксенов	\N	\N	\N	\N	\N	2026-07-21 15:44:10.593699	аксенов василий
953	Татьяна	\N	Москвина	\N	\N	\N	\N	\N	2026-07-21 15:44:10.593699	москвина татьяна
954	Андрей	\N	Битов	\N	\N	\N	\N	\N	2026-07-21 15:44:10.593699	битов андрей
955	Андрей	\N	Макаревич	\N	\N	\N	\N	\N	2026-07-21 15:44:10.593699	макаревич андрей
956	Павел	\N	Крусанов	\N	\N	\N	\N	\N	2026-07-21 15:44:10.593699	крусанов павел
957	Андрей	\N	Аствацатуров	\N	\N	\N	\N	\N	2026-07-21 15:44:10.593699	аствацатуров андрей
958	Евгений	\N	Водолазкин	\N	\N	\N	\N	\N	2026-07-21 15:44:10.593699	водолазкин евгений
960	Татьяна	\N	Толстая	\N	\N	\N	\N	\N	2026-07-21 15:44:10.593699	толстая татьяна
961	Денис	\N	Драгунский	\N	\N	\N	\N	\N	2026-07-21 15:44:10.593699	драгунский денис
962	Михаил	\N	Шишкин	\N	\N	\N	\N	\N	2026-07-21 15:44:10.593699	шишкин михаил
963	Александр	\N	Иличевский	\N	\N	\N	\N	\N	2026-07-21 15:44:10.593699	иличевский александр
964	Сергей	\N	Шаргунов	\N	\N	\N	\N	\N	2026-07-21 15:44:10.593699	шаргунов сергей
965	Захар	\N	Прилепин	\N	\N	\N	\N	\N	2026-07-21 15:44:10.593699	прилепин захар
966	Майя	\N	Кучерская	\N	\N	\N	\N	\N	2026-07-21 15:44:10.593699	кучерская майя
967	Дмитрий	\N	Горчев	\N	\N	\N	\N	\N	2026-07-21 15:44:10.593699	горчев дмитрий
968	Александр	\N	Терехов	\N	\N	\N	\N	\N	2026-07-21 15:44:10.593699	терехов александр
969	Марина	\N	Степнова	\N	\N	\N	\N	\N	2026-07-21 15:44:10.593699	степнова марина
970	Юрий	\N	Казаков	\N	\N	\N	\N	\N	2026-07-21 15:44:10.593699	казаков юрий
972	Алексей	\N	Портнов	\N	\N	\N	\N	\N	2026-07-21 15:44:10.593699	портнов алексей
974	Томас	\N	Рид	\N	\N	\N	\N	\N	2026-07-21 15:45:10.815939	рид томас
976	Томас Майн	\N	Рид	\N	\N	\N	\N	\N	2026-07-21 15:45:11.063954	рид томас майн
979	Стивен Р.	\N	Кови	\N	\N	\N	\N	\N	2026-07-21 15:45:37.997756	кови стивен р.
982	Карлос	\N	Кастанеда	\N	\N	\N	\N	\N	2026-07-21 15:46:12.559763	кастанеда карлос
1059	Максим	\N	Дорофеев	\N	\N	\N	\N	\N	2026-07-21 16:02:01.162583	дорофеев максим
1062	Карлос	\N	КАСТАНЕДА	\N	\N	\N	\N	\N	2026-07-21 16:02:51.827047	кастанеда карлос
1064	Карло́с	\N	Кастанеда	\N	\N	\N	\N	\N	2026-07-21 16:04:13.680014	кастанеда карло́с
1067	Карлос	\N	Каста́неда	\N	\N	\N	\N	\N	2026-07-21 16:06:12.236656	каста́неда карлос
1071	Валерий	\N	Осинский	\N	\N	\N	\N	\N	2026-07-21 16:07:37.655156	осинский валерий
1084		\N	Empty ISBN Author	\N	\N	\N	\N	\N	2026-07-21 16:09:01.078846	empty isbn author 
1087		\N	Title Change Author	\N	\N	\N	\N	\N	2026-07-21 16:09:01.117888	title change author 
75	Неизвестный	\N	автор	\N	\N	\N	\N	\N	2026-07-10 16:56:32.558163	автор неизвестный
1088		\N	EmptyStr Author	\N	\N	\N	\N	\N	2026-07-21 16:09:01.141347	emptystr author 
1089		\N	Corrupt Author	\N	\N	\N	\N	\N	2026-07-21 16:09:01.15954	corrupt author 
1090		\N	Author To Remove	\N	\N	\N	\N	\N	2026-07-21 16:09:01.657874	author to remove 
1091		\N	Genre Remove Author	\N	\N	\N	\N	\N	2026-07-21 16:09:01.677387	genre remove author 
79	Елена Петровна	1	Блаватская	\N	\N	\N	\N	\N	2026-07-10 16:57:18.750363	блаватская елена петровна
983	Graham	\N	Campbell	\N	\N	\N	\N	\N	2026-07-21 15:46:29.363865	campbell graham
984	Taylor	\N	Otwell	\N	\N	\N	\N	\N	2026-07-21 15:46:29.363865	otwell taylor
985	Леонид	\N	Андреев	\N	\N	\N	\N	\N	2026-07-21 15:46:29.382077	андреев леонид
986	Игорь	\N	Ашманов	\N	\N	\N	\N	\N	2026-07-21 15:46:29.412558	ашманов игорь
987	Joel	\N	Grus	\N	\N	\N	\N	\N	2026-07-21 15:46:45.354403	grus joel
988	Бенджамин	\N	Франклин	\N	\N	\N	\N	\N	2026-07-21 15:46:45.395766	франклин бенджамин
990	Джермен	\N	Гвишиани	\N	\N	\N	\N	\N	2026-07-21 15:46:45.572924	гвишиани джермен
991	Тигран	\N	Хачатуров	\N	\N	\N	\N	\N	2026-07-21 15:46:45.572924	хачатуров тигран
992	Вадим	\N	Кириченко	\N	\N	\N	\N	\N	2026-07-21 15:46:45.572924	кириченко вадим
994	Елизавета Петровна	\N	Блаватская	\N	\N	\N	\N	\N	2026-07-21 15:48:18.549859	блаватская елизавета петровна
997	Фридрих	\N	Ницше	\N	\N	\N	\N	\N	2026-07-21 15:49:38.704073	ницше фридрих
998	Бхагаван	\N	Раджниш	\N	\N	\N	\N	\N	2026-07-21 15:50:20.035534	раджниш бхагаван
1006	Бхагаван	\N	Раджшиш	\N	\N	\N	\N	\N	2026-07-21 15:50:20.286397	раджшиш бхагаван
1007	Project Management	\N	Institute	\N	\N	\N	\N	\N	2026-07-21 15:51:00.123321	institute project management
1008		\N	Inc.	\N	\N	\N	\N	\N	2026-07-21 15:51:00.123321	inc. 
1013	Оливье	\N	Сибони	\N	\N	\N	\N	\N	2026-07-21 15:51:00.455477	сибони оливье
1014	Даниэль	\N	Канеман	\N	\N	\N	\N	\N	2026-07-21 15:51:00.455477	канеман даниэль
1015	Касс	\N	Санстейн	\N	\N	\N	\N	\N	2026-07-21 15:51:00.455477	санстейн касс
1016	Лев	\N	Толстой	\N	\N	\N	\N	\N	2026-07-21 15:51:00.588068	толстой лев
1019	Флоринда	\N	Доннер	\N	\N	\N	\N	\N	2026-07-21 15:51:39.683694	доннер флоринда
1020	Гевин	\N	Кеннеди	\N	\N	\N	\N	\N	2026-07-21 15:51:39.729419	кеннеди гевин
1021	РОББИНС	\N	Энтони	\N	\N	\N	\N	\N	2026-07-21 15:52:16.550161	энтони роббинс
1022	Кьелл А.	\N	Нордстрем	\N	\N	\N	\N	\N	2026-07-21 15:53:01.242092	нордстрем кьелл а.
1023	Йонас	\N	Риддерстрале	\N	\N	\N	\N	\N	2026-07-21 15:53:01.242092	риддерстрале йонас
1026	Ральф Х. Абернэти/Ричард	\N	Фрэм	\N	\N	\N	\N	\N	2026-07-21 15:55:27.376747	фрэм ральф х. абернэти/ричард
1027	Владимир	\N	Герасичев	\N	\N	\N	\N	\N	2026-07-21 15:55:27.399981	герасичев владимир
1028	Иван	\N	Маурах	\N	\N	\N	\N	\N	2026-07-21 15:55:27.399981	маурах иван
1029	Арсен	\N	Рябуха	\N	\N	\N	\N	\N	2026-07-21 15:55:27.399981	рябуха арсен
1030	Тайша	\N	Абеляр	\N	\N	\N	\N	\N	2026-07-21 15:56:12.463689	абеляр тайша
1031	Аркадий и Борис	\N	Стругацкие	\N	\N	\N	\N	\N	2026-07-21 15:56:12.504626	стругацкие аркадий и борис
1032	Florinda	\N	Donner	\N	\N	\N	\N	\N	2026-07-21 15:57:02.278284	donner florinda
1035	Георгий Иванович	\N	Гурджиев	\N	\N	\N	\N	\N	2026-07-21 15:59:05.869285	гурджиев георгий иванович
1036	Евгений	\N	Войскунский	\N	\N	\N	\N	\N	2026-07-21 15:59:05.914317	войскунский евгений
1037	Владимир	\N	Покровский	\N	\N	\N	\N	\N	2026-07-21 15:59:05.914317	покровский владимир
1038	Андрей	\N	Саломатов	\N	\N	\N	\N	\N	2026-07-21 15:59:05.914317	саломатов андрей
1039	Алексей	\N	Андреев	\N	\N	\N	\N	\N	2026-07-21 15:59:05.914317	андреев алексей
151	Carlos	\N	Castaneda	\N	\N	\N	\N	\N	2026-07-10 17:13:13.581428	castaneda carlos
1040	Любовь	\N	Лукина	\N	\N	\N	\N	\N	2026-07-21 15:59:05.914317	лукина любовь
1042	Эдуард	\N	Геворкян	\N	\N	\N	\N	\N	2026-07-21 15:59:05.914317	геворкян эдуард
247		\N	Платон	\N	\N	\N	\N	\N	2026-07-10 17:16:17.677118	платон 
249	Сергей	\N	Полотовский	\N	\N	\N	\N	\N	2026-07-10 17:16:18.055594	полотовский сергей
250	Роман	\N	Козак	\N	\N	\N	\N	\N	2026-07-10 17:16:18.055594	козак роман
251	Чалдини	\N	Роберт	\N	\N	\N	\N	\N	2026-07-10 17:16:50.755504	роберт чалдини
252	Владимир	\N	Серкин	\N	\N	\N	\N	\N	2026-07-10 17:16:50.771508	серкин владимир
255	Сергей	\N	Сиротин	\N	\N	\N	\N	\N	2026-07-10 17:16:50.80986	сиротин сергей
257	Марк Сидоний	\N	Фалкс	\N	\N	\N	\N	\N	2026-07-10 17:16:50.8385	фалкс марк сидоний
258	Джерри	\N	Тонер	\N	\N	\N	\N	\N	2026-07-10 17:16:50.8385	тонер джерри
259	Нассим	\N	Талеб	\N	\N	\N	\N	\N	2026-07-10 17:17:30.231262	талеб нассим
260	Клаус	\N	Шваб	\N	\N	\N	\N	\N	2026-07-10 17:18:05.551176	шваб клаус
261	Федор	\N	Достоевский	\N	\N	\N	\N	\N	2026-07-10 17:18:05.645105	достоевский федор
263	Луценко А.	\N	И.	\N	\N	\N	\N	\N	2026-07-10 17:18:42.731667	и. луценко а.
264		\N	Multi Author	\N	\N	\N	\N	\N	2026-07-12 12:18:02.496071	multi author 
266		\N	Updater	\N	\N	\N	\N	\N	2026-07-12 12:18:02.515928	updater 
267		\N	Deleter	\N	\N	\N	\N	\N	2026-07-12 12:18:02.532796	deleter 
268		\N	Original Author	\N	\N	\N	\N	\N	2026-07-12 12:18:02.548976	original author 
269	New	\N	Author	\N	\N	\N	\N	\N	2026-07-12 12:18:02.560149	author new
270		\N	UniqueField Author	\N	\N	\N	\N	\N	2026-07-12 12:18:02.573093	uniquefield author 
1044	Егор	\N	Лавров	\N	\N	\N	\N	\N	2026-07-21 15:59:05.914317	лавров егор
272		\N	Author One	\N	\N	\N	\N	\N	2026-07-12 12:18:02.606438	author one 
273		\N	Author Two	\N	\N	\N	\N	\N	2026-07-12 12:18:02.612695	author two 
1045	Владимир	\N	Гопман	\N	\N	\N	\N	\N	2026-07-21 15:59:05.914317	гопман владимир
1046	Г. И.	\N	Гюрджиев	\N	\N	\N	\N	\N	2026-07-21 15:59:47.799375	гюрджиев г. и.
1047	Борис	\N	Акунин	\N	\N	\N	\N	\N	2026-07-21 16:00:29.791841	акунин борис
1048	Гуру Рам-Дас	\N	Бхай	\N	\N	\N	\N	\N	2026-07-21 16:01:07.795816	бхай гуру рам-дас
1049	Екатерина Константиновна	\N	Блаватская	\N	\N	\N	\N	\N	2026-07-21 16:02:00.708856	блаватская екатерина константиновна
279		\N	Tag Remove Author	\N	\N	\N	\N	\N	2026-07-12 12:18:03.031074	tag remove author 
280		\N	Nil Author Person	\N	\N	\N	\N	\N	2026-07-12 12:18:03.047847	nil author person 
281		\N	Initial Author	\N	\N	\N	\N	\N	2026-07-12 12:18:03.06264	initial author 
282	Added	\N	Author	\N	\N	\N	\N	\N	2026-07-12 12:18:03.068663	author added
302		\N	ыва	\N	\N	\N	\N	\N	2026-07-12 13:08:36.622782	ыва 
303	ыва	\N	аы	\N	\N	\N	\N	\N	2026-07-12 19:31:55.342291	аы ыва
1050	Георгий	\N	Гурджиев	\N	\N	\N	\N	\N	2026-07-21 16:02:00.721419	гурджиев георгий
1053	Георгий	\N	ГУРДЖИЕВ	\N	\N	\N	\N	\N	2026-07-21 16:02:00.936861	гурджиев георгий
800	Колин	\N	Уилсон	\N	\N	\N	\N	\N	2026-07-20 10:38:30.159804	уилсон колин
801	Карл	\N	Юнг	\N	\N	\N	\N	\N	2026-07-20 12:09:39.385954	юнг карл
1058		\N	Лао-цзы	\N	\N	\N	\N	\N	2026-07-21 16:02:01.050855	лао-цзы 
\.


--
-- Data for Name: read_list; Type: TABLE DATA; Schema: public; Owner: -
--

COPY public.read_list (listname, bookname, author, priority, author_id, book_id, user_id, comment, status, created_at, id, updated_at, synced_at, deleted, looking_for) FROM stdin;
default			1	\N	\N	1		Не заполнено	2026-07-21 20:43:42.058032	b3da2a8f-0453-4d55-a81a-8e653d148526	2026-07-21 20:43:42.058032	\N	f	Нет
default			4	\N	\N	1		Не заполнено	2026-07-24 13:21:05.147135	05e4e0e1-e8cf-4b3f-a300-9ea2399907be	2026-07-24 13:21:05.147135	\N	f	Нет
default	Тестовая книга	Тест	6	\N	\N	1		Не заполнено	2026-07-27 10:21:20.281791	5dd735d5-7471-4b0e-b3d3-dc0b1ad9209b	2026-07-27 10:21:20.281791	\N	f	Да, локально
default	Нер13		83	\N	\N	1		Не заполнено	2026-07-16 09:41:21.60786	f7a86aab-1ce1-4ebe-a6b3-c37a257b30bd	2026-07-16 13:05:05.599799	\N	t	Нет
default	Трам2		80	\N	\N	1		Не заполнено	2026-07-15 16:01:15.70749	19587fe2-2ff5-4e2c-b067-ee28ab0ccea7	2026-07-16 13:05:07.726104	\N	t	Нет
default	Книга без флага	Автор	0	\N	\N	1		Не заполнено	2026-07-27 10:21:20.297086	940b5111-687b-4db6-b1c9-ada9cd57bb23	2026-07-27 10:21:37.478629	\N	f	Да, по федерации
default	Dvbc		87	\N	\N	1		Не заполнено	2026-07-16 13:19:26.308538	52a92299-f192-4083-a8dd-aa8eff255299	2026-07-16 13:24:05.501765	\N	t	Нет
default	Aaa		86	\N	\N	1		Не заполнено	2026-07-16 13:18:07.780286	6a405d09-afc6-4bd6-8af5-9f166faaa477	2026-07-16 13:24:06.52376	\N	t	Нет
default	Tecn	Fgbb	85	\N	\N	1		Не заполнено	2026-07-16 13:17:59.26455	83a32a58-66f7-44c0-830b-2c82579c3699	2026-07-16 13:24:07.138782	\N	t	Нет
default	Ttt	Fhh	70	\N	\N	1	F	Не заполнено	2026-07-15 15:52:15.626783	1f967836-9cdc-4c96-b352-7d4afd91926e	2026-07-16 13:24:07.853629	\N	t	Нет
default	Bbb1		0	\N	\N	1		Не заполнено	2026-07-16 13:18:29.669536	1fe3c58e-ee21-4bb0-8804-2e79461a781d	2026-07-16 13:24:11.134146	\N	t	Нет
default	KGBT+ (КГБТ+)	Пелевин Виктор	5	2	781	1		Не заполнено	2026-07-24 15:36:07.592301	2a58a77b-b3ad-49db-b132-2f19368e8899	2026-07-24 15:36:07.592301	\N	f	Нет
default	Gbbn1		95	\N	\N	1		Не заполнено	2026-07-16 13:39:50.689815	60ac8c7d-360f-4428-99af-d4efe75b5c5f	2026-07-16 14:35:42.737243	\N	t	Нет
default	Тест	X	7	\N	\N	1		Не заполнено	2026-07-27 10:21:37.494937	301cdce9-58f8-44a5-bf6d-fcff656d1a6f	2026-07-27 10:44:15.394792	\N	f	Да, по федерации
default	S.N.U.F.F.12	Пелевин Виктор	89	2	\N	1		Не заполнено	2026-07-16 13:24:51.096039	0d8147c4-06b5-4d08-ab85-b07d2a373fbe	2026-07-16 13:37:44.762094	\N	t	Нет
default	Ааа		99	\N	\N	1		Не заполнено	2026-07-16 18:31:00.366757	b03a98de-86fe-46c2-aa37-a539c741fb25	2026-07-16 18:32:34.137462	\N	t	Нет
default		Андреев Алексей	1	1039	\N	582		Не заполнено	2026-07-21 20:58:36.869127	f7f81aa1-1463-49c5-9c67-cf0f6ea6d178	2026-07-22 09:27:06.258158	\N	t	Нет
default	New		101	\N	\N	1		Не заполнено	2026-07-16 18:53:48.976751	b77b8105-33ac-4f48-b298-af8bfaccdfcc	2026-07-16 18:54:10.035042	\N	t	Нет
default	косметическая химия.pdf	автор Неизвестный	3	75	198	582		Прочитано	2026-07-22 09:22:31.721328	b356d3dd-c88c-4664-b811-0abcf41021e5	2026-07-22 09:24:50.294327	\N	f	Нет
default	Xbbu		100	\N	\N	1		Не заполнено	2026-07-16 19:03:01.295767	7a5580aa-7475-4e47-8bae-893f5b6f4d3b	2026-07-16 19:04:26.286665	\N	t	Нет
default	Fybv		101	\N	\N	1		Не заполнено	2026-07-16 19:03:11.651859	319bef3f-d874-40e9-8f23-7a7d0e12d28e	2026-07-16 19:04:27.235888	\N	t	Нет
default	Rghjjn		102	\N	\N	1		Не заполнено	2026-07-16 19:03:20.958167	97e35317-6441-4c1f-8839-66c68fb4971a	2026-07-16 19:04:28.780049	\N	t	Нет
default	Testbook		90	\N	\N	1		Не заполнено	2026-07-16 13:37:44.935459	250140a3-f6dd-43ae-bf60-36bab45c9293	2026-07-16 14:40:14.080962	\N	t	Нет
default			50	\N	\N	1		Не заполнено	2026-07-16 15:07:13.73212	42fed8ac-e05a-43d1-9575-98955ace1733	2026-07-16 18:28:23.565478	\N	t	Нет
default			100	\N	\N	1		Не заполнено	2026-07-16 15:07:09.884663	402a57ae-5aa5-4dd0-b243-bdcc5467b9ab	2026-07-16 18:28:23.609191	\N	t	Нет
default	Fh		101	\N	\N	1		Не заполнено	2026-07-16 17:22:04.589617	8539f990-aade-4331-88a2-bd9db0191f44	2026-07-16 18:29:02.418891	\N	t	Нет
default	Dgbvf		97	\N	\N	1		Не заполнено	2026-07-16 14:40:43.324674	3d94cf4e-2cb2-43ae-94b6-d83aa2168c2e	2026-07-16 14:42:26.957447	\N	t	Нет
default			105	\N	\N	1		Не заполнено	2026-07-16 18:29:12.304706	0fd8675d-019f-4b9e-b176-b47e24660b44	2026-07-16 18:30:34.779263	\N	t	Нет
default			104	\N	\N	1		Не заполнено	2026-07-16 18:29:10.035445	0a876daa-ba81-4d31-9a7a-8916da590d18	2026-07-16 18:30:34.802677	\N	t	Нет
default			103	\N	\N	1		Не заполнено	2026-07-16 18:29:08.123263	a3bd0f1a-938f-41a1-a305-97b2a5e566ac	2026-07-16 18:30:34.826834	\N	t	Нет
default			108	\N	\N	1		Не заполнено	2026-07-16 18:30:49.250453	7231a7aa-ce95-4dc7-bd89-bf5dec21850f	2026-07-16 18:31:04.873308	\N	t	Нет
default			107	\N	\N	1		Не заполнено	2026-07-16 18:30:47.475226	e826c3b0-3fe3-48f6-87f4-fc2cf565be4f	2026-07-16 18:31:05.337805	\N	t	Нет
default			106	\N	\N	1		Не заполнено	2026-07-16 18:30:45.576465	2970dd56-1569-425d-a1de-a81dcfe238c4	2026-07-16 18:31:05.766532	\N	t	Нет
default	Пелевин В. - Круть (Трансгуманизм - 4) - 2024.a4.pdf	автор Неизвестный	99	75	183	1		Не заполнено	2026-07-16 19:04:40.66777	971b4914-166f-4da6-a599-e9903ea6dfea	2026-07-17 05:06:27.85532	\N	t	Нет
default	Gjnfr		109	\N	\N	1		Не заполнено	2026-07-17 04:59:44.305552	d14ae58a-9e49-40b6-855a-151fc2d98d86	2026-07-17 05:07:38.715209	\N	t	Нет
default	Corrupted ISBN Test		102	\N	381	1		Не заполнено	2026-07-16 20:28:55.151365	0ff4d413-5333-4655-829e-1bfbfa8a5d17	2026-07-17 05:07:38.794716	\N	t	Нет
default	Tttt		114	\N	\N	1		Не заполнено	2026-07-17 05:07:58.070449	1818d3f5-8e9f-48d2-8e3d-3858857387b7	2026-07-17 05:08:05.035731	\N	t	Нет
default	Who by fire	Пелевин Виктор	115	2	102	1		Не заполнено	2026-07-17 06:30:27.714598	735559cc-9005-4be4-8811-c7a39a539c36	2026-07-17 06:30:43.577018	\N	t	Нет
default			118	\N	\N	1		Не заполнено	2026-07-17 06:37:53.147476	bf9ec107-8319-4210-8d74-ad9f342616d5	2026-07-17 06:38:29.745413	\N	t	Нет
default			117	\N	\N	1		Не заполнено	2026-07-17 06:37:51.707008	f1949cf4-8c4e-41bb-a9cf-b6e82b06514b	2026-07-17 06:38:30.26315	\N	t	Нет
default	выаывфаыв		121	\N	\N	1		Не заполнено	2026-07-17 09:09:45.737702	0c070fe3-35a0-49f8-a4f4-9e459d9515dc	2026-07-17 09:10:33.620221	\N	t	Нет
default			1	\N	\N	1		Не заполнено	2026-07-17 10:10:48.898625	7544d3a3-f402-4c54-8551-09d29d9cad34	2026-07-17 10:11:00.551109	\N	t	Нет
default	Test1234		101	\N	\N	1		Не заполнено	2026-07-17 10:17:59.756871	cd0c0742-afb0-441b-8e74-7fe1ebd35430	2026-07-17 11:38:47.334586	\N	t	Нет
default	Fhthbf		100	\N	\N	1		Не заполнено	2026-07-17 10:17:59.702326	b4ca4942-2749-4ead-a2b3-cf7136e7d0db	2026-07-17 11:38:47.373034	\N	t	Нет
default	Fhh		99	\N	\N	1		Не заполнено	2026-07-17 10:11:50.791811	a3e07a07-2082-4d9c-ab61-6486b893a858	2026-07-17 11:38:47.410795	\N	t	Нет
default	Апртда	Роашлв	102	\N	\N	1	Тест	Не заполнено	2026-07-17 19:45:19.072653	97f41ba4-25fc-4f96-b995-c9fe58d1073d	2026-07-17 20:20:07.282294	\N	t	Нет
default	Ррррр		101	\N	\N	1		Не заполнено	2026-07-17 19:45:19.037798	111d229e-ccce-4342-9935-40e1a385471a	2026-07-17 20:20:09.021029	\N	t	Нет
default			100	\N	\N	1		Не заполнено	2026-07-17 20:27:23.528838	4b3f121e-c084-4847-a99f-54093ce8d8b2	2026-07-17 20:27:26.069082	\N	t	Нет
default	Tru	Empty ISBN Author	110	\N	\N	1		Не заполнено	2026-07-17 05:00:31.709618	a303b3a3-4140-4de3-9ae1-30ba14b1e249	2026-07-17 05:07:38.501994	\N	t	Нет
default			3	\N	\N	1		Не заполнено	2026-07-22 09:29:16.922095	3d937cf5-687b-4bc8-bfd4-9aa0f6a77f7a	2026-07-22 09:29:16.922095	\N	f	Нет
default			2	\N	\N	1		Не заполнено	2026-07-21 20:43:43.708189	dc1f9033-5ec5-4428-b273-848b0afbf3f7	2026-07-22 09:29:19.79469	\N	t	Нет
default			9	\N	\N	1		Не заполнено	2026-07-18 20:50:14.867128	81f90ec8-01b3-46ad-acb7-a9dc0b3c2102	2026-07-18 20:51:49.718865	\N	t	Нет
default			8	\N	\N	1		Не заполнено	2026-07-18 20:49:56.744064	3fe827c2-6356-442b-a89c-e8f8be6655e1	2026-07-18 20:51:49.843149	\N	t	Нет
default			1	\N	\N	1		Не заполнено	2026-07-20 08:28:25.706134	fd1532e3-df1e-4848-9633-eb3058fc363e	2026-07-20 08:31:14.469631	\N	t	Нет
default	Testasync		125	\N	\N	1		Не заполнено	2026-07-17 10:24:29.083935	40e98db8-a28e-4457-a5c4-3ad8494a7cd8	2026-07-17 10:25:05.665444	\N	t	Нет
default	testsite		124	\N	\N	1		Не заполнено	2026-07-17 10:19:59.282174	fe29a703-2462-4269-9245-1546ea1103ae	2026-07-17 10:25:06.485351	\N	t	Нет
default	Testmob		102	\N	\N	1		Не заполнено	2026-07-17 10:21:09.390629	8f195acb-f37f-4805-a8d5-aeeb929bb904	2026-07-17 10:25:08.051519	\N	t	Нет
default	Bookc fhcthbf rtg	Fgb	102	\N	\N	1		Не заполнено	2026-07-17 11:38:47.472367	5186dd9c-bda4-4d45-96ae-94ae67e5e8d4	2026-07-17 11:38:47.472367	\N	t	Нет
default			126	\N	\N	1		Не заполнено	2026-07-17 11:39:33.487046	5e90e86b-c4ac-43ff-93fc-73fd0f436141	2026-07-17 12:47:48.416185	\N	t	Нет
Смотреть			101	\N	\N	1		Не заполнено	2026-07-17 14:55:38.256421	eea3b556-17ce-479f-84ec-b5a4f04c7882	2026-07-17 14:55:53.679144	\N	t	Нет
default	Dvh		102	\N	\N	1		Не заполнено	2026-07-17 15:36:24.615421	c09daf16-540c-47f7-a079-1b0879fb1a52	2026-07-17 15:36:29.736421	\N	t	Нет
default	Test		103	\N	\N	1		Не заполнено	2026-07-17 15:36:24.753713	cdc29379-9146-4508-8e06-c3a2cd50d125	2026-07-17 15:36:31.068079	\N	t	Нет
default	Add Author Test	Initial Author	100	281	233	1		Не заполнено	2026-07-17 11:39:17.589033	fb178572-3646-45aa-84f5-2386e2d477bf	2026-07-17 15:36:36.290302	\N	t	Нет
default	Book Two	Author Two	1	273	446	1		Не заполнено	2026-07-17 15:34:48.30766	513f1edc-b7ce-4bd8-8c44-ffde719aeef6	2026-07-17 15:37:31.690311	\N	t	Нет
default	Corrupted ISBN Test		1	\N	381	1		Не заполнено	2026-07-17 12:47:17.794807	69fc9ee7-3319-4c46-b363-f3981409ca34	2026-07-17 15:37:32.765292	\N	t	Нет
default	Fbn		103	\N	\N	1		Не заполнено	2026-07-17 20:20:01.172707	414f8b94-e8f1-4de4-a433-4062e5cddc08	2026-07-17 20:20:10.454167	\N	t	Нет
default			103	\N	\N	1		Не заполнено	2026-07-17 20:27:29.586407	0761f4c8-b924-448b-ab0c-57265f1be36b	2026-07-17 20:40:24.523363	\N	t	Нет
default		Толстая Татьяна	13	\N	\N	1		Не заполнено	2026-07-20 08:18:28.416784	d9ff5fd9-7d7e-43e2-aa26-245e157509f8	2026-07-20 08:19:18.471944	\N	t	Нет
default	Corrupted ISBN Testd		2	\N	\N	582		Не заполнено	2026-07-22 09:22:27.215963	cbe90778-2029-4ee5-91e2-a96151502aa6	2026-07-23 13:18:43.855824	\N	f	Нет
default			102	\N	\N	1		Не заполнено	2026-07-17 20:27:28.240932	1ebe4dd8-8db2-490f-a4c4-ebd2e6ff0041	2026-07-17 21:32:26.435033	\N	t	Нет
default			101	\N	\N	1		Не заполнено	2026-07-17 20:27:25.050668	b0f95473-fda2-47f9-bbc7-13f1286b1502	2026-07-17 21:32:26.567804	\N	t	Нет
default		Author Two	100	273	\N	1		Не заполнено	2026-07-18 06:58:10.393236	ad5f5d69-36d2-49f3-9fb2-0b0abc34d56b	2026-07-18 06:58:51.66218	\N	t	Нет
default			10	\N	\N	1		Не заполнено	2026-07-18 20:52:40.147745	36f5e661-30f0-402d-945d-c37bf5100067	2026-07-20 08:19:17.969797	\N	t	Нет
default			11	\N	\N	1		Не заполнено	2026-07-18 20:52:40.316502	240d6323-0157-4db4-b780-7c4b4fd00a11	2026-07-20 08:19:18.084747	\N	t	Нет
default	Аьь		14	\N	\N	1		Не заполнено	2026-07-20 08:18:47.798719	9575643f-70d5-4cd1-8856-6f3e65f728b7	2026-07-20 08:19:18.566576	\N	t	Нет
default	Акпо	Аи	4	\N	\N	1		Не заполнено	2026-07-20 08:30:38.961862	f9608948-de6b-4c04-a333-80cdd92af442	2026-07-20 08:30:38.961862	\N	t	Нет
default			2	\N	\N	1		Не заполнено	2026-07-20 08:29:16.302255	13bc02d9-ce47-416f-816f-faf242fe3de1	2026-07-20 08:31:17.763792	\N	t	Нет
default			3	\N	\N	1		Не заполнено	2026-07-20 08:29:16.449877	db9024fd-8d02-406d-92f2-4d341c26d1e1	2026-07-20 08:31:18.352708	\N	t	Нет
default	Кккк		5	\N	\N	1		Не заполнено	2026-07-20 08:30:38.802649	3198c960-2ff7-44eb-8d76-9d0318d86ef0	2026-07-20 08:31:18.674862	\N	t	Нет
\.


--
-- Data for Name: reading_progress; Type: TABLE DATA; Schema: public; Owner: -
--

COPY public.reading_progress (id, edition_id, current_position, total_positions, percentage, device, started_at, finished_at, rating, notes, updated_at) FROM stdin;
\.


--
-- Data for Name: refresh_tokens; Type: TABLE DATA; Schema: public; Owner: -
--

COPY public.refresh_tokens (id, user_id, token_hash, device_name, device_fingerprint, created_at) FROM stdin;
2	1	e26efd092469057f2c3f7602e08181bcf5fabeb04b7728d4164c78dcc64080b1	Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/149.0.0.0 Sa	fp_rwb5r8	2026-07-11 17:38:55.337141
3	1	a18f34f2e1aeb8ed6f67b555fa5fc12145d365c6f98b4c677d81939bcd527fd7	Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/150.0.0.0 Sa	fp_fo6tvh	2026-07-12 11:58:21.032864
4	1	e07159f279b9635931013e276bafcf9af15e73060e3956cfad875390658f4bf0	Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/150.0.0.0 Sa	fp_shpo2c	2026-07-13 10:17:11.449073
5	1	360c2c378c9d286ebc391aaf28afde69974e5026024d1c33c1ad41978a1d4eee	Mozilla/5.0 (Linux; Android 14; SM-S908B Build/UP1A.231005.007; wv) AppleWebKit/537.36 (KHTML, like 	fp_6vanvd	2026-07-13 10:29:12.905417
6	1	e7eb7d4dc0575c9471c9608c9b1a84978cb22ec06fd99599a23dd239e9b8457f	Mozilla/5.0 (Linux; Android 14; SM-S908B Build/UP1A.231005.007; wv) AppleWebKit/537.36 (KHTML, like 	fp_6vanvd	2026-07-13 11:33:15.070994
7	1	7f3d9e35a79ca09ed692dfc4e7c750ae33f57230e16ef484feee9d34f7f06e76	Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/150.0.0.0 Sa	fp_shpo2c	2026-07-13 12:53:14.774424
8	1	9d5c8300400f82d634978ecb130acc258f1c6b9618f2a6062644af3d7868c071	Mozilla/5.0 (Linux; Android 14; SM-S908B Build/UP1A.231005.007; wv) AppleWebKit/537.36 (KHTML, like 	fp_6vanvd	2026-07-14 10:59:40.557498
9	1	01dab28ff9d1fbe99dd3e385e2184482253afdc93f2cce8707d7678b01e14a8f	Mozilla/5.0 (Linux; Android 14; SM-S908B Build/UP1A.231005.007; wv) AppleWebKit/537.36 (KHTML, like 	fp_6vanvd	2026-07-14 11:46:05.283413
10	1	615c585f51935ea5abddad69edd3ead2d7195be234cff8ab4eaa56190110625d	Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/150.0.0.0 Sa	fp_fo6tvh	2026-07-14 13:44:58.97751
11	1	0b0be13e3bf9cee61be14118b8db198c5e9a46fa7bca104ee2c06b5094327b17	Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/150.0.0.0 Sa	fp_fo6tvh	2026-07-14 13:45:49.98946
12	1	bd8398883f2021fa47f6a123c1b6b311e6cd5ace4c069621f333d5cf3d5e782b	Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/150.0.0.0 Sa	fp_fo6tvh	2026-07-14 13:46:05.231351
13	36	56e771b917fd7076145fc4d813096c413f75ab52a9185ec60fd850357962ed10	Unknown device		2026-07-15 08:50:02.041378
14	1	031af29c89409e38dcf0388b757bda650cbdc5d5b08c3ff5c437f5f0f420d5ea	Mozilla/5.0 (Linux; Android 14; SM-S908B Build/UP1A.231005.007; wv) AppleWebKit/537.36 (KHTML, like 	fp_9ublin	2026-07-15 11:13:06.303625
15	1	b354be7d4f974a294c7cb078998b7ca1bcf645a0dce65cd9354873ec82a7e717	Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/150.0.0.0 Sa	fp_fo6tvh	2026-07-15 13:19:11.348666
16	1	f949d3bdd2a880290ef25f9c9fbc9374cc9cab1c2ba82ce50bd69f420624f594	Mozilla/5.0 (Linux; Android 14; SM-S908B Build/UP1A.231005.007; wv) AppleWebKit/537.36 (KHTML, like 	fp_9ublin	2026-07-15 13:26:21.148415
17	1	0fd2d2d2921930b02636dcaf72eefc5da5f4c1083f0d02148123b85f110e72b1	Mozilla/5.0 (Linux; Android 14; SM-S908B Build/UP1A.231005.007; wv) AppleWebKit/537.36 (KHTML, like 	fp_9ublin	2026-07-15 13:32:45.814976
18	1	ab1bf76f02228d6c8138658854c8cacc2fe97da1e3c494906b7da080e5ee6566	Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/145.0.0.0 Safari/537.3	fp_wrj9ao	2026-07-15 15:38:14.477574
19	1	59da03c984b489f74b6ca6ac458ec3499fbf7236a6980214b3cb2323a5b15776	Mozilla/5.0 (Linux; Android 14; SM-S908B Build/UP1A.231005.007; wv) AppleWebKit/537.36 (KHTML, like 	fp_9ublin	2026-07-15 15:44:06.38567
20	1	a2d48d71d049347296c00eaebcb2f3280cee86199e48cff19bd301a29b065f49	Mozilla/5.0 (Linux; Android 14; SM-S908B Build/UP1A.231005.007; wv) AppleWebKit/537.36 (KHTML, like 	fp_9ublin	2026-07-15 15:51:09.551273
21	1	557bdbd50b2c733552178a5e27faf82c67b64ea0fbb8f27a7c7e88d007e839d7	Mozilla/5.0 (Linux; Android 14; SM-S908B Build/UP1A.231005.007; wv) AppleWebKit/537.36 (KHTML, like 	fp_9ublin	2026-07-15 15:53:48.268742
22	1	7177f3779a41aaf98f30b6959b99ea3daaed0c78250c644e81cacf5749bcf050	Mozilla/5.0 (Linux; Android 14; SM-S908B Build/UP1A.231005.007; wv) AppleWebKit/537.36 (KHTML, like 	fp_9ublin	2026-07-16 13:04:41.791251
23	1	92538a523265ff9963fede9e66a92dcce9a18a59c7520f56e73d6a265d8a6bfe	Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/150.0.0.0 Sa	fp_fo6tvh	2026-07-16 13:24:21.41354
24	1	a03f27f7e9b999ec8eda295eb2c8bb6591070331bed99f46a77d01df011204c7	Mozilla/5.0 (Linux; Android 14; SM-S908B Build/UP1A.231005.007; wv) AppleWebKit/537.36 (KHTML, like 	fp_9ublin	2026-07-16 14:35:14.4259
25	1	e98b9bc7897ea42deef29cf02378d8093503f0575658f0a59f400fd7be8112ee	Mozilla/5.0 (Linux; Android 14; SM-S908B Build/UP1A.231005.007; wv) AppleWebKit/537.36 (KHTML, like 	fp_9ublin	2026-07-16 14:37:54.170269
26	1	800c73d2254493f037bdc37cb555f9cdbf8bb67694485c0c985421b85377794c	Mozilla/5.0 (Linux; Android 14; SM-S908B Build/UP1A.231005.007; wv) AppleWebKit/537.36 (KHTML, like 	fp_9ublin	2026-07-16 17:21:06.658686
27	1	40ad6052169737f90c9e91b42d16581de25bb2efc5d94798726e6f4c40260896	Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/145.0.0.0 Safari/537.3	fp_wrj9ao	2026-07-16 17:21:40.245071
28	1	6e4f1e0a13f79fb85310858f176dec1c5c5f54354a7d462407fcb8585031c7c2	Mozilla/5.0 (Linux; Android 14; SM-S908B Build/UP1A.231005.007; wv) AppleWebKit/537.36 (KHTML, like 	fp_9ublin	2026-07-17 05:06:12.027176
29	1	63afb40cffd53f0169e55eb98231e3674c95f4ce2af7308601b54e95d08b7be9	Mozilla/5.0 (Linux; Android 14; SM-S908B Build/UP1A.231005.007; wv) AppleWebKit/537.36 (KHTML, like 	fp_9ublin	2026-07-17 06:37:23.29413
30	1	81eda585640de520dca7106c95e60c1d0b967e1355935453dfc04d7225804823	Mozilla/5.0 (Linux; Android 14; SM-S908B Build/UP1A.231005.007; wv) AppleWebKit/537.36 (KHTML, like 	fp_9ublin	2026-07-17 06:40:33.407008
31	1	865989d4e62a33e0881fb20107072b4c5bba56fc78acd107c5aaa86480e4e1c7	Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/150.0.0.0 Sa	fp_fo6tvh	2026-07-17 06:41:02.42946
32	1	c31876d75fb1b905b160cfc6af960eed3d4372a6e49e5da50d07e23c78033061	Mozilla/5.0 (Linux; Android 14; SM-S908B Build/UP1A.231005.007; wv) AppleWebKit/537.36 (KHTML, like 	fp_9ublin	2026-07-17 06:42:59.023775
33	1	ba5e6fe1af37f92b96ad6269be3d24cd275bea8cacba9fb598388a98b09a4bc9	Mozilla/5.0 (Linux; Android 14; SM-S908B Build/UP1A.231005.007; wv) AppleWebKit/537.36 (KHTML, like 	fp_9ublin	2026-07-17 09:09:12.340614
34	1	df4da250a13251518843c65feec5e6d7c6d25801b924c9b1193a38824550b373	Mozilla/5.0 (Linux; Android 14; SM-S908B Build/UP1A.231005.007; wv) AppleWebKit/537.36 (KHTML, like 	fp_9ublin	2026-07-17 10:10:35.493743
35	1	c6aa814ee27312fe7bdc4b6ac6cad734df3d5e8ee2f806a242b033a7d3bf0581	Mozilla/5.0 (Linux; Android 14; SM-S908B Build/UP1A.231005.007; wv) AppleWebKit/537.36 (KHTML, like 	fp_9ublin	2026-07-17 12:47:09.853177
36	1	86763ce383c4d288829350d43cf1d3edf849066d74ead53c089e06682b1f81ad	Mozilla/5.0 (Linux; Android 14; SM-S908B Build/UP1A.231005.007; wv) AppleWebKit/537.36 (KHTML, like 	fp_9ublin	2026-07-17 12:48:22.659424
37	1	ae3ed0417608af8a08ced633f9154163c09baeae3a75339b749126ecddd50e64	Mozilla/5.0 (Linux; Android 14; SM-S908B Build/UP1A.231005.007; wv) AppleWebKit/537.36 (KHTML, like 	fp_9ublin	2026-07-17 12:50:21.948649
122	1	6760111565871177b4266fd356040e0d6221d47aa5a3c8015fd639f01e3dff74	Unknown device		2026-07-25 15:18:53.186087
38	1	0dfa482506b4bb6adffa19cfcef90dc721c727b08e2c3f56ebf8caf02e0638ae	Mozilla/5.0 (Linux; Android 14; SM-S908B Build/UP1A.231005.007; wv) AppleWebKit/537.36 (KHTML, like 	fp_9ublin	2026-07-17 12:54:27.267604
39	1	a3d33f11c269e41965ecd8a6ed01c13b50627e10cbaacb5ed87cb808c94a7831	Mozilla/5.0 (Linux; Android 14; SM-S908B Build/UP1A.231005.007; wv) AppleWebKit/537.36 (KHTML, like 	fp_9ublin	2026-07-17 15:34:36.423335
40	1	d8ab7515acd87a2d9778202a66e3f3a3fe417e760a4cdadae04b2123160f3df4	Mozilla/5.0 (Linux; Android 14; SM-S908B Build/UP1A.231005.007; wv) AppleWebKit/537.36 (KHTML, like 	fp_9ublin	2026-07-17 15:42:40.984914
41	1	20b697e6f3b8e67c14f54130e7abfe676003612c0c827190b1884bbdfca97d6c	Mozilla/5.0 (Linux; Android 14; SM-S908B Build/UP1A.231005.007; wv) AppleWebKit/537.36 (KHTML, like 	fp_9ublin	2026-07-17 15:43:56.418139
42	1	bb3c12ef2e82bc45aba607edcceb3892f3907cfc589d510316782bd0d7a0ca26	Mozilla/5.0 (Linux; Android 14; SM-S908B Build/UP1A.231005.007; wv) AppleWebKit/537.36 (KHTML, like 	fp_9ublin	2026-07-17 15:44:44.993572
43	1	e42ca38c768818e05da19558f2c0f547fe38a86320ff99b6b243391e7564dfa1	Mozilla/5.0 (Linux; Android 14; SM-S908B Build/UP1A.231005.007; wv) AppleWebKit/537.36 (KHTML, like 	fp_9ublin	2026-07-17 15:50:58.44793
44	1	cbb167667452902af9dd6c88d0317ecd6d33c7bce4709be133ffdcdbb9b06b30	Mozilla/5.0 (Linux; Android 14; SM-S908B Build/UP1A.231005.007; wv) AppleWebKit/537.36 (KHTML, like 	fp_9ublin	2026-07-17 15:51:09.816167
45	1	1d368b70609f0a39f5f3cca3e160e7188f597030847c416f5276fe8227bb2d3b	Mozilla/5.0 (Linux; Android 14; SM-S908B Build/UP1A.231005.007; wv) AppleWebKit/537.36 (KHTML, like 	fp_9ublin	2026-07-17 15:51:26.198882
46	1	f295223b980022ced96b4524b89659f37b81f53afb6d0c0ecb67467bfc1e8a71	Mozilla/5.0 (Linux; Android 14; SM-S908B Build/UP1A.231005.007; wv) AppleWebKit/537.36 (KHTML, like 	fp_9ublin	2026-07-17 19:32:28.385913
47	1	f50fc8ed1d3d7e2e4002f4af6ea3b87bc5e3ef3ac645f675eb9a7c19dbe737fa	Mozilla/5.0 (Linux; Android 14; SM-S908B Build/UP1A.231005.007; wv) AppleWebKit/537.36 (KHTML, like 	fp_9ublin	2026-07-17 19:32:40.676248
48	411	7cc1a4124f7ed9a3a0f363eee138623c59b36c5440a8c0e3d7fe4a6ebb9e1816	Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/150.0.0.0 Sa	fp_fo6tvh	2026-07-17 19:34:23.794484
49	411	bb2e4279627050cab882cfed30767735c261a714386c5cdac4ca498c3796162c	Mozilla/5.0 (Linux; Android 14; SM-S908B Build/UP1A.231005.007; wv) AppleWebKit/537.36 (KHTML, like 	fp_9ublin	2026-07-17 19:35:01.338168
50	1	4bd9bfcd043cbea38a4e5abcb3214474430d8d34fb423d8125fa519c73a47ff9	Mozilla/5.0 (Linux; Android 14; SM-S908B Build/UP1A.231005.007; wv) AppleWebKit/537.36 (KHTML, like 	fp_9ublin	2026-07-17 19:35:17.963045
51	1	0726651c0e92278bba8b579d3899f81357591b094b44b22ee3959add3e0cb838	Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/150.0.0.0 Sa	fp_fo6tvh	2026-07-17 19:35:27.690535
52	1	42398fec838d4d02fb07ac6c53505b0270e2a51a95a05f080677b9b9d7fb3f19	Mozilla/5.0 (Linux; Android 14; SM-S908B Build/UP1A.231005.007; wv) AppleWebKit/537.36 (KHTML, like 	fp_9ublin	2026-07-17 19:55:30.70155
53	1	879005739aa8c330f46ead8716e0f49303a96eddaaf77360c8ed49d6754c1d56	Mozilla/5.0 (Linux; Android 14; SM-S908B Build/UP1A.231005.007; wv) AppleWebKit/537.36 (KHTML, like 	fp_9ublin	2026-07-17 20:19:54.378555
54	1	a28864ed755c6fcda82a1add23c8f9bfc516e41226c68f885b716985042b29c9	Mozilla/5.0 (Linux; Android 14; SM-S908B Build/UP1A.231005.007; wv) AppleWebKit/537.36 (KHTML, like 	fp_9k6a6n	2026-07-17 20:27:17.613179
55	1	4f5a48e23175e26da092610479aabbc7fa719d78335b2403425b52ccf60dd79b	Mozilla/5.0 (Linux; Android 14; SM-S908B Build/UP1A.231005.007; wv) AppleWebKit/537.36 (KHTML, like 	fp_9k6a6n	2026-07-17 20:27:38.505133
56	1	0b391e70e3631a02165dc8da6ee6db7ee1175c5548169a8fe3c25deb19506325	Mozilla/5.0 (Linux; Android 14; SM-S908B Build/UP1A.231005.007; wv) AppleWebKit/537.36 (KHTML, like 	fp_9k6a6n	2026-07-17 20:40:21.174194
57	1	2ae8f52c560eb89b88d9bf26f1ad0f860775f177067bc18465ee7df9fb973122	Mozilla/5.0 (Linux; Android 14; SM-S908B Build/UP1A.231005.007; wv) AppleWebKit/537.36 (KHTML, like 	fp_9k6a6n	2026-07-17 20:40:33.959427
58	1	28fcb8e0c9e14740eab1e5b523a271aceca0c5c55870420432d23afbd0bdfd9d	Mozilla/5.0 (Linux; Android 14; SM-S908B Build/UP1A.231005.007; wv) AppleWebKit/537.36 (KHTML, like 	fp_9k6a6n	2026-07-17 21:31:40.119045
59	1	1bca25ba1858c488e58353e81eb6c254b916da0cc28c4d8896764f8cd34b25b3	Mozilla/5.0 (Linux; Android 14; SM-S908B Build/UP1A.231005.007; wv) AppleWebKit/537.36 (KHTML, like 	fp_9k6a6n	2026-07-18 06:57:41.758933
60	1	b514a0725a8b2b188d23cded5c8a6d957df97a5da151dd46abd144e6a4de637f	Mozilla/5.0 (Linux; Android 14; SM-S908B Build/UP1A.231005.007; wv) AppleWebKit/537.36 (KHTML, like 	fp_9k6a6n	2026-07-18 07:30:32.317221
61	1	31789312cd7a1f24483f800dc3dc899ffd1b20c3be40fa5cf30d457b24463867	Mozilla/5.0 (Linux; Android 14; SM-S908B Build/UP1A.231005.007; wv) AppleWebKit/537.36 (KHTML, like 	fp_9k6a6n	2026-07-18 08:53:43.322112
62	1	bdbdde0dc8e3641272aef38432c5e143040297f6cf3988976388d7398a4e2066	Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/145.0.0.0 Safari/537.3	fp_wrj9ao	2026-07-18 13:03:00.332629
63	1	351c4e819e1018e81c8f7fdc196573c1e23fce741ecd12a9d60548d95780bff1	Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/145.0.0.0 Safari/537.3	fp_wrj9ao	2026-07-18 13:03:00.391878
64	1	067ccf24131086fe40c70d98f0a4ad256187c1e62c21b370e13fa6b8fad4b0f3	Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/145.0.0.0 Safari/537.3	fp_wrj9ao	2026-07-18 19:59:24.683588
65	1	534fa88927d635a9f32aec1532784d2539f75424625f4e406d38baabc2c70dc1	Mozilla/5.0 (Linux; Android 14; SM-S908B Build/UP1A.231005.007; wv) AppleWebKit/537.36 (KHTML, like 	fp_9ublin	2026-07-18 20:02:42.870964
66	1	a9b3b081bfc384abd8091f4985e9db249a5ca192e194563f2775fc3ec79f0aee	Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/150.0.0.0 Sa	fp_fo6tvh	2026-07-20 07:10:11.329396
67	1	8e525fb33df6de2c83e1fd160d062d05ec4c5689cb1d832938eca5b3ce92dd12	Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/150.0.0.0 Sa	fp_fo6tvh	2026-07-20 07:10:11.436966
68	1	dea4a8e89f60df075fcc19e9812bde73834a3e4ef6eb89b7321d6b99a50c4206	Mozilla/5.0 (Linux; Android 14; SM-S908B Build/UP1A.231005.007; wv) AppleWebKit/537.36 (KHTML, like 	fp_9ublin	2026-07-20 10:33:29.822548
69	1	cc87203da99c8051996494c98d94e63a11577f8dde25b9bd775e893f5d8a5a82	Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/145.0.0.0 Safari/537.3	fp_wrj9ao	2026-07-20 10:38:08.195097
70	1	a5268d81d86c54d117d56b048e0bbfa8f32b4b9d3b8bad710e0148a1ae02c0dc	Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/145.0.0.0 Safari/537.3	fp_wrj9ao	2026-07-20 10:38:08.241151
71	1	e3c190bd4a03220512c33315ae3572c35c9362339e0ad593659e87a5c2cc593d	Mozilla/5.0 (Linux; Android 14; SM-S908B Build/UP1A.231005.007; wv) AppleWebKit/537.36 (KHTML, like 	fp_9ublin	2026-07-20 12:08:57.75258
72	1	3525379c82d30027e11ce9f32b2397dd7d1a66360d5268687e5b05e97f16eea5	Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/150.0.0.0 Sa	fp_fo6tvh	2026-07-21 11:45:11.076074
73	1	9e2b793438d27b3f1a2533cfe7676ce1ac6eddf640a21854129f0463d5f0b4db	Mozilla/5.0 (Linux; Android 14; SM-S908B Build/UP1A.231005.007; wv) AppleWebKit/537.36 (KHTML, like 	fp_w0g6v8	2026-07-21 12:13:27.047436
74	1	5c11e802e17ffb0978c8be3b90a521c4e3b1ebc11ecb7618d22b11036e15d5c2	Mozilla/5.0 (Linux; Android 14; SM-S908B Build/UP1A.231005.007; wv) AppleWebKit/537.36 (KHTML, like 	fp_w0g6v8	2026-07-21 12:26:28.145013
75	1	c327b4587b0398fd46f4182b0a94f75d217deab5e500dc559a7e92a85988d23b	Mozilla/5.0 (Linux; Android 14; SM-S908B Build/UP1A.231005.007; wv) AppleWebKit/537.36 (KHTML, like 	fp_w0g6v8	2026-07-21 12:48:47.360224
76	1	14d82a1b8a8e4143ecc5178d10afe873e1a99c52a679a722463855ca3fe5ebce	Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/150.0.0.0 Sa	fp_fo6tvh	2026-07-21 15:33:10.424585
77	1	2f79fb93bed6b15208d5aff502b9caf8191af0cf2c22b0afc0917fddcd4d9090	Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/150.0.0.0 Sa	fp_fo6tvh	2026-07-21 15:33:47.257892
78	1	a1a12ca9524a5cce82f433ff827f0589cb79750e3a343a7e781f4a0587f1afca	Unknown device		2026-07-21 15:39:31.776354
79	1	3f55adf8c475928e054a3530f2b8bfe71834af4072332e8aab732914aac5a97a	Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/150.0.0.0 Sa	fp_fo6tvh	2026-07-21 20:08:25.150618
80	1	50a5880e8bc9543846951c36525f5d8ba8308a8fd157566eb40f262f817fea1d	Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/150.0.0.0 Sa	fp_fo6tvh	2026-07-21 20:44:32.308338
81	1	d9051e707b5b7753651c334f4f0d17067309ae196fe04e8a741af23e327290ac	Unknown device		2026-07-21 20:45:36.350725
82	582	963c5c0938cfe4efacedfae680771ca4f22123a54119db0f360e60cb7678acd4	Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/150.0.0.0 Sa	fp_fo6tvh	2026-07-21 20:58:13.051225
83	1	896efdcdc61b1bf38e2a553e7d8ecdf7415852814f932a3e1cd2b8e193cd85cb	Unknown device		2026-07-21 21:10:08.499471
84	583	e50dd75f64876344d7f4472bf3e2bd88b3bad80fe4a454d2b2974145f1d535ab	Unknown device		2026-07-21 21:12:01.28864
85	583	1f99f180091bd6227a37b3edebe00868874c8d8fbe2abce24245728b3748783a	Unknown device		2026-07-21 21:24:08.176055
86	582	bbec93156aa68e16e61a2664c28d6c2c793ced33be06476cb64b68cce9bb0d05	Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/150.0.0.0 Sa	fp_fo6tvh	2026-07-22 09:22:22.958159
87	1	ea7b6e956932f42f365fc105bc91629578c838d1b2010f309fa12f16619b78b2	Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/150.0.0.0 Sa	fp_fo6tvh	2026-07-22 09:27:36.924013
88	583	3664523501efb20814965a1e3b1c35795199a0f5b6c814c0f43356926e3da1ef	Unknown device		2026-07-22 09:34:46.93844
89	1	22de6efa41c89333178964e05d8d032834e5b23a47db4f5e80a6d31104eed5a4	Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/150.0.0.0 Sa	fp_fo6tvh	2026-07-23 07:13:23.996633
90	582	ac645d241ca48d07271f841c07215559cb3223d75ab2b8164f0dfd75f4d5a2c1	Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/150.0.0.0 Sa	fp_fo6tvh	2026-07-23 07:14:03.827393
91	1	3d4ab8b4f2e4355b9490bd581806f535ad2511ce6a341f4218506d5c0c5c5334	Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/150.0.0.0 Sa	fp_fo6tvh	2026-07-23 07:14:53.391608
92	582	5db6f5260fd50ab04bcd897728fa9d01955566ed42b0d785c52ed94a062e3001	Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/150.0.0.0 Sa	fp_fo6tvh	2026-07-23 13:18:37.759161
93	1	0f73d7eb2d49704a02286b2f50d4529ffe08d3674bb3db57c724a6f82822a4cb	Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/150.0.0.0 Sa	fp_fo6tvh	2026-07-23 13:20:17.801533
94	1	ef8b59c6f311b37668b9ebe3c28efd620c0ef795b1c0973c2e39c04b56163717	Mozilla/5.0 (Linux; Android 14; SM-S908B Build/UP1A.231005.007; wv) AppleWebKit/537.36 (KHTML, like 	fp_w0g6v8	2026-07-23 15:55:27.557138
95	1	7b775d1dd3f712f03eaa47a5d3dcdada9cf9a387b8a3ee234c64d419d1c556fb	Unknown device		2026-07-24 13:13:48.706983
96	1	819b43d351faabd5acb609677a01fdb6ca80e99aa7312e9412968e340f8dbc8a	Unknown device		2026-07-24 13:16:42.377383
97	1	c7cd6d4fc4f61eca383892acd728dc115e9e8d9faf4cc3dd5500c6af9f07d4e4	Unknown device		2026-07-24 13:16:57.711111
98	1	71bddae156982881bf843fb77b677b481fd7bd991ec4f9d4b0c6740523d92d0a	Unknown device		2026-07-24 13:17:07.482181
99	1	cc9a87ab2dad9d3b8b191ccf13cb904f078f9eb25fd5c5e93694aaef3b91986d	Unknown device		2026-07-24 13:17:16.05208
100	1	67565ccdd492d929510523e41ca207fa78ee688aa72d0ba593c475d5bb45f3f5	Unknown device		2026-07-24 13:17:49.741308
101	1	0a58f139accfa6d4c9573ff70f1209338879d15cbe6c49ad19661520f73a62ef	Unknown device		2026-07-24 13:18:18.52749
102	1	6b7ea206d23d73ce710ba5d87ce3815ec620908a13d6cba3e8f67dd1bdabb226	Unknown device		2026-07-24 13:18:38.636624
103	1	42a3e09b621ce4e440919b30d2bd484d657cfe078d6f55e569dd71fdfe516af1	Unknown device		2026-07-24 13:19:08.433119
104	1	a0422fa714b4e274cb4b972b781c58461f3014ec97f98ffe5375f98e1c82a03b	Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/150.0.0.0 Sa	fp_fo6tvh	2026-07-24 13:20:59.500457
105	1	161f1c9724f60fb15e650ab51bcebbd21db9175ae75ca7391da0d84ce5e46a6e	Mozilla/5.0 (Linux; Android 14; SM-S908B Build/UP1A.231005.007; wv) AppleWebKit/537.36 (KHTML, like 	fp_w0g6v8	2026-07-24 13:26:32.040011
106	1	bae7dd5d3f689fe3fb72ca335b9defd442001501706c2e0e28c98fba0354158a	Unknown device		2026-07-24 13:36:35.977626
107	1	cb37bf1ddfe9c8888cd47419f001d69664fd97e2feb63e83fe3242abfe32aa5b	Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/150.0.0.0 Sa	fp_fo6tvh	2026-07-24 13:53:32.744575
108	1	728e9c11c2bd6b0f4d4860df668855f14f3dcf9234e8e12e2fa2a88c9602aaa6	Unknown device		2026-07-24 15:07:58.293261
109	1	8947a576947587308d3a9ddb92e38494dc8d3974eda23fd9568e23af3040a6a7	Unknown device		2026-07-24 15:08:26.172464
110	1	6065851ba061f5504437ec88ab7e9d8300d010b581c479f8131712ab7e3de479	Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/145.0.0.0 Safari/537.3	fp_wrj9ao	2026-07-24 15:35:47.887769
111	1	95e20d773f02b7f12708d43767fd3ab1aa9faf57842b8b15fa40b2f43f7f2461	Unknown device		2026-07-24 15:41:18.604734
112	1	a18ab4fbf94e83d24bd2e27d8ae02045139cd4b15d753829e7c7c72ac8a6aa79	Unknown device		2026-07-24 15:46:41.542409
113	1	f4e6653fec5a2aabee77fa4fda68e962cf9f34c18b08a1f1233c0a6fc20ececd	Unknown device		2026-07-24 15:53:07.491885
114	1	9203f83463b9a534f1f7b7215b3ae9715f2e5e8280fa7927695648db2d23fcbf	Unknown device		2026-07-24 15:54:38.904108
115	1	fdf3f4a36c5d1a595020795754c285ab36da127b72fc9d682512752c63c260ca	Unknown device		2026-07-24 19:06:24.517807
116	1	12e34303763b70da97c47a3a68214bf19e94c819a0af09b4e0b9e0780c0c692e	Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/150.0.0.0 Sa	fp_fo6tvh	2026-07-24 19:10:26.775476
117	1	c3ce4862a1d6bea1b4567bbf1aa25dbfabc1b92339d4fac648516b62e14e5568	Unknown device		2026-07-24 19:12:54.007295
118	1	708ee0a54857aa5851661f5db06e464fda484a4bddfec8b1f239cfd3ec9d0408	Unknown device		2026-07-24 19:13:13.433198
119	1	fd89f26494c3b686b5b769529734a58eb35bdab1e4243b56f925846a49038c03	Mozilla/5.0 (Linux; Android 12; DBY-W09 Build/HUAWEIDBY-W09; wv) AppleWebKit/537.36 (KHTML, like Gec	fp_2o44dv	2026-07-25 14:14:40.127756
120	1	f5214d2eb64307ea7ce3009671bd45b9ff220ba7121b1c2a7352265ab520ef92	Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/150.0.0.0 Sa	fp_fo6tvh	2026-07-25 15:07:09.670324
121	1	5baef3ecd89995c1d49db8565915e9a839da58080f7d87753d08d28d6d580f39	Unknown device		2026-07-25 15:18:36.34839
123	1	104976144926f8c5f1c2524542b554dcd9da06e1331ae8e54db275ee1dfa7fe6	Unknown device		2026-07-25 15:19:02.811898
124	1	a6fd5aeb38e2e23a6bd0165bc85653f55e2b0c0d84a9d5c032b5ef62fd107b91	Unknown device		2026-07-25 15:25:04.383666
125	1	d4c6ff5425dfae8478f7584c8a5f58b3360a4c3d751db82a5c2f58b4bcb1bd92	Unknown device		2026-07-25 15:30:07.182833
126	1	b2d3a29a5a81b2bcfc52dd40f44c61ac181d3476c1e90b146b01a331e48a69fd	Mozilla/5.0 (Linux; Android 12; DBY-W09 Build/HUAWEIDBY-W09; wv) AppleWebKit/537.36 (KHTML, like Gec	fp_kpambl	2026-07-25 15:43:19.657523
127	1	a2e68207025a3164efe4f0a476af9844cc7c905fa8bbe931d30a036dc8ec129d	Unknown device		2026-07-25 15:48:10.000231
128	1	df4e3c77d0144e4611ed71abd4b287ae8ac12a285881f74f20c8a41930e3eca7	Unknown device		2026-07-25 15:48:21.084153
129	1	d1c2d407ca901c5d54ebc66099c113fc8c80c902ba9c9f497bd22ca737d444fc	Unknown device		2026-07-25 15:55:15.103663
130	1	cf6e40522b9984c14500e2f031188c9eeb430b7cbc945ded59e232bec550a41e	Unknown device		2026-07-25 15:55:21.970058
131	1	4e6d38c01c9e26bad7e834d5f6076a13442601cbfb9f900ae7f49e300923173b	Unknown device		2026-07-25 16:35:18.689142
132	1	af82b3d1a66a2261d34b6f5f3f6128691d36e3d5687735e4f63eb355bc9cecb3	Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/150.0.0.0 Sa	fp_fo6tvh	2026-07-26 19:35:40.91128
133	1	69a0267772e2242efef3c011892be1c635712f3166c94528027af690accb36ab	Unknown device		2026-07-27 07:11:36.04647
134	1	f26fe2ae1df6c38944a1bdb521ff10abaac7431c5637d89cd39f4e995e18cd3a	Unknown device		2026-07-27 07:11:46.555486
135	1	5dc77cb886da6bac98991a84b76b8e40ab18f3fe2e607b0b70022cdd22e352c0	Unknown device		2026-07-27 07:16:13.894194
136	1	96d1265db4f65449f3a9b804e9b5e0f97040ec59270ef95c39e97212213e4abb	Unknown device		2026-07-27 07:16:21.744499
137	1	12369b96ef26ddff4d5cd0fe63833909231a345d54eddb838decdabd6f8fcba3	Unknown device		2026-07-27 10:21:20.270223
138	1	e9b8f34a44de59a5cdb2ced4a934600eae3104a2ad1be1c8458c4baa3236863a	Unknown device		2026-07-27 10:21:37.467854
\.


--
-- Data for Name: settings; Type: TABLE DATA; Schema: public; Owner: -
--

COPY public.settings (key, value, updated_at) FROM stdin;
backup_dir		2026-07-16 14:37:13.602247
\.


--
-- Data for Name: shelf_tokens; Type: TABLE DATA; Schema: public; Owner: -
--

COPY public.shelf_tokens (id, token, edition_id, created_at) FROM stdin;
\.


--
-- Data for Name: tags; Type: TABLE DATA; Schema: public; Owner: -
--

COPY public.tags (id, name, color, description) FROM stdin;
1	testtag_remove	\N	\N
\.


--
-- Data for Name: toc_entries; Type: TABLE DATA; Schema: public; Owner: -
--

COPY public.toc_entries (id, edition_id, parent_id, level, title, "position", sort_order) FROM stdin;
\.


--
-- Data for Name: user_books; Type: TABLE DATA; Schema: public; Owner: -
--

COPY public.user_books (id, user_id, edition_id, status, review, rating, date_started, date_read, created_at, updated_at) FROM stdin;
42	582	198	Прочитано		\N	\N	\N	2026-07-22 09:24:50.294327	2026-07-22 09:24:50.294327
62	1	369	Прочитано		\N	\N	2026-07-25	2026-07-25 15:42:16.943322	2026-07-25 15:42:16.943322
63	1	1116	Прочитано		\N	\N	2026-07-25	2026-07-25 15:42:19.970699	2026-07-25 15:42:19.970699
\.


--
-- Data for Name: user_devices; Type: TABLE DATA; Schema: public; Owner: -
--

COPY public.user_devices (id, user_id, device_name, device_fingerprint, created_at) FROM stdin;
1	1	Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/149.0.0.0 Sa	fp_rwb5r8	2026-07-11 17:38:55.335661
4	1	Mozilla/5.0 (Linux; Android 14; SM-S908B Build/UP1A.231005.007; wv) AppleWebKit/537.36 (KHTML, like 	fp_6vanvd	2026-07-13 10:29:12.903718
92	1	Mozilla/5.0 (Linux; Android 12; DBY-W09 Build/HUAWEIDBY-W09; wv) AppleWebKit/537.36 (KHTML, like Gec	fp_2o44dv	2026-07-25 14:14:40.126044
2	1	Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/150.0.0.0 Sa	fp_fo6tvh	2026-07-12 11:58:21.031501
16	1	Mozilla/5.0 (X11; Linux x86_64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/145.0.0.0 Safari/537.3	fp_wrj9ao	2026-07-15 15:38:14.476704
\.


--
-- Data for Name: users; Type: TABLE DATA; Schema: public; Owner: -
--

COPY public.users (id, username, password_hash, email, role, created_at, updated_at) FROM stdin;
411	admin2	$2a$10$XYQucFSbJKL7G3RrCJ/CDelF7xaSt2Vh6TG3TI01EZNzPRgE0iz3y	\N	admin	2026-07-17 19:34:16.007737	2026-07-17 19:34:16.007737
205	rl_iso2_djzwi8jb2gcb	$2a$10$dummyhash	\N	viewer	2026-07-16 09:51:03.447823	2026-07-16 09:51:03.447823
6	rl_iso2_djwl4lps99qw	$2a$10$dummyhash	\N	viewer	2026-07-12 12:18:02.960387	2026-07-12 12:18:02.960387
69	rl_iso2_djz5bybpcesf	$2a$10$dummyhash	\N	viewer	2026-07-15 12:33:21.067823	2026-07-15 12:33:21.067823
141	rl_iso2_djzjxxhkhasn	$2a$10$dummyhash	\N	viewer	2026-07-16 00:00:18.795919	2026-07-16 00:00:18.795919
344	rl_iso2_dk069uxjpm1o	$2a$10$dummyhash	\N	viewer	2026-07-16 17:30:18.019678	2026-07-16 17:30:18.019678
282	rl_iso2_dk02p76nmh3f	$2a$10$dummyhash	\N	viewer	2026-07-16 14:42:15.714663	2026-07-16 14:42:15.714663
601	edit	$2a$10$KiZkBvwmd5L4i21hBGMQ7.9P9G0szoJd7r2Y2d3OMgVFgLo2YsxIu	\N	editor	2026-07-22 09:29:43.80163	2026-07-23 07:13:48.734079
13	rl_iso2_djwl4zdxefg2	$2a$10$dummyhash	\N	viewer	2026-07-12 12:18:32.718376	2026-07-12 12:18:32.718376
416	rl_iso2_dk15xty4gb6s	$2a$10$dummyhash	\N	viewer	2026-07-17 21:27:15.464431	2026-07-17 21:27:15.464431
76	rl_iso2_djz5ftmfk2mm	$2a$10$dummyhash	\N	viewer	2026-07-15 12:38:24.289209	2026-07-15 12:38:24.289209
484	rl_iso2_dk3mdx3w3ynv	$2a$10$dummyhash	\N	viewer	2026-07-20 18:45:53.842053	2026-07-20 18:45:53.842053
552	rl_iso2_dk4docr7j38f	$2a$10$dummyhash	\N	viewer	2026-07-21 16:09:01.512844	2026-07-21 16:09:01.512844
691	rl_iso2_dk6wali8oej5	$2a$10$dummyhash	\N	viewer	2026-07-24 15:09:44.46488	2026-07-24 15:09:44.46488
759	rl_iso2_dk6x8rk3xuim	$2a$10$dummyhash	\N	viewer	2026-07-24 15:54:22.020131	2026-07-24 15:54:22.020131
218	rl_iso2_djzwifyxcken	$2a$10$dummyhash	\N	viewer	2026-07-16 09:51:19.629721	2026-07-16 09:51:19.629721
83	rl_iso2_djz6dqboz8fr	$2a$10$dummyhash	\N	viewer	2026-07-15 13:22:41.491198	2026-07-15 13:22:41.491198
623	rl_iso2_dk5rmlv7tq8o	$2a$10$dummyhash	\N	viewer	2026-07-23 07:17:40.113356	2026-07-23 07:17:40.113356
827	rl_iso2_dk727p9li5px	$2a$10$dummyhash	\N	viewer	2026-07-24 19:48:04.216376	2026-07-24 19:48:04.216376
895	rl_iso2_dk7rbx2dlw7l	$2a$10$dummyhash	\N	viewer	2026-07-25 15:29:02.398463	2026-07-25 15:29:02.398463
980	rl_iso2_dk8ssfsoozp9	$2a$10$dummyhash	\N	viewer	2026-07-26 20:50:18.064424	2026-07-26 20:50:18.064424
1048	rl_iso2_dk99upcpmavf	$2a$10$dummyhash	\N	viewer	2026-07-27 10:12:34.463084	2026-07-27 10:12:34.463084
90	rl_iso2_djz8901tihuf	$2a$10$dummyhash	\N	viewer	2026-07-15 14:50:33.060923	2026-07-15 14:50:33.060923
33	rl_iso2_djz0gmd44uml	$2a$10$dummyhash	\N	viewer	2026-07-15 08:44:21.302967	2026-07-15 08:44:21.302967
160	rl_iso2_djzqleew5e12	$2a$10$dummyhash	\N	viewer	2026-07-16 05:13:04.674692	2026-07-16 05:13:04.674692
298	rl_iso2_dk03d4eownx1	$2a$10$dummyhash	\N	viewer	2026-07-16 15:13:30.410139	2026-07-16 15:13:30.410139
36	testuser	$2a$10$xtmF2.ynvqTsP662NmetJ.vmh2ke7khX6OaIDCsvPXSwcbaTdqrcS	\N	viewer	2026-07-15 08:50:02.040256	2026-07-15 08:50:02.040256
360	rl_iso2_dk069y0mvswh	$2a$10$dummyhash	\N	viewer	2026-07-16 17:30:24.73675	2026-07-16 17:30:24.73675
97	rl_iso2_djz8e9lgrby9	$2a$10$dummyhash	\N	viewer	2026-07-15 14:57:25.660668	2026-07-15 14:57:25.660668
433	rl_iso2_dk1kxtfucor4	$2a$10$dummyhash	\N	viewer	2026-07-18 09:12:31.007684	2026-07-18 09:12:31.007684
234	rl_iso2_djzwiib6jujg	$2a$10$dummyhash	\N	viewer	2026-07-16 09:51:24.724328	2026-07-16 09:51:24.724328
501	rl_iso2_dk49u9df8m3j	$2a$10$dummyhash	\N	viewer	2026-07-21 13:08:39.894268	2026-07-21 13:08:39.894268
48	rl_iso2_djz3ppdbl6kl	$2a$10$dummyhash	\N	viewer	2026-07-15 11:17:16.453035	2026-07-15 11:17:16.453035
569	rl_iso2_dk4jro7d899o	$2a$10$dummyhash	\N	viewer	2026-07-21 20:55:28.186443	2026-07-21 20:55:28.186443
708	rl_iso2_dk6wysejrwk4	$2a$10$dummyhash	\N	viewer	2026-07-24 15:41:20.219067	2026-07-24 15:41:20.219067
640	rl_iso2_dk6ttjc3gegu	$2a$10$dummyhash	\N	viewer	2026-07-24 13:13:25.329151	2026-07-24 13:13:25.329151
176	rl_iso2_djzqlh92u1ju	$2a$10$dummyhash	\N	viewer	2026-07-16 05:13:10.853421	2026-07-16 05:13:10.853421
55	rl_iso2_djz3wez23m88	$2a$10$dummyhash	\N	viewer	2026-07-15 11:26:02.371901	2026-07-15 11:26:02.371901
313	rl_iso2_dk03h6ynj3q4	$2a$10$dummyhash	\N	viewer	2026-07-16 15:18:49.427386	2026-07-16 15:18:49.427386
776	rl_iso2_dk71btpfaz21	$2a$10$dummyhash	\N	viewer	2026-07-24 19:06:26.227235	2026-07-24 19:06:26.227235
844	rl_iso2_dk7md2juxqgo	$2a$10$dummyhash	\N	viewer	2026-07-25 11:35:27.153961	2026-07-25 11:35:27.153961
115	rl_iso2_djz8hljkhvp1	$2a$10$dummyhash	\N	viewer	2026-07-15 15:01:46.759885	2026-07-15 15:01:46.759885
62	rl_iso2_djz5arc42y4s	$2a$10$dummyhash	\N	viewer	2026-07-15 12:31:47.490887	2026-07-15 12:31:47.490887
912	rl_iso2_dk7rptprelbu	$2a$10$dummyhash	\N	viewer	2026-07-25 15:47:12.203462	2026-07-25 15:47:12.203462
997	rl_iso2_dk8t2fw8cbb0	$2a$10$dummyhash	\N	viewer	2026-07-26 21:03:21.920449	2026-07-26 21:03:21.920449
250	rl_iso2_dk01jzio6imw	$2a$10$dummyhash	\N	viewer	2026-07-16 13:48:26.096212	2026-07-16 13:48:26.096212
381	rl_iso2_dk08dj2bi2nc	$2a$10$dummyhash	\N	viewer	2026-07-16 19:09:07.863323	2026-07-16 19:09:07.863323
450	rl_iso2_dk3hm0odrj7n	$2a$10$dummyhash	\N	viewer	2026-07-20 15:01:22.97514	2026-07-20 15:01:22.97514
582	view	$2a$10$k1n9ahYCLwZPsRag9e7Q9eJryS3I65I7Y7GHPYnlmecxX6kOGoS9K	\N	viewer	2026-07-21 20:58:01.580492	2026-07-21 20:58:01.580492
583	testviewer	$2a$10$eQCMZzhlCPfOx6oMnQ2yUuMyajjXhUsBvxpLHSis.UkT3fO1b.XSq	\N	viewer	2026-07-21 21:11:12.180888	2026-07-21 21:11:12.180888
1065	rl_iso2_dk99yiko6g9v	$2a$10$dummyhash	\N	viewer	2026-07-27 10:17:33.163574	2026-07-27 10:17:33.163574
518	rl_iso2_dk4c6vbdp2cd	$2a$10$dummyhash	\N	viewer	2026-07-21 14:59:10.249727	2026-07-21 14:59:10.249727
329	rl_iso2_dk03h9dliorm	$2a$10$dummyhash	\N	viewer	2026-07-16 15:18:54.684549	2026-07-16 15:18:54.684549
588	rl_iso2_dk4kctyeqjum	$2a$10$dummyhash	\N	viewer	2026-07-21 21:23:06.352882	2026-07-21 21:23:06.352882
725	rl_iso2_dk6x2yoe5s85	$2a$10$dummyhash	\N	viewer	2026-07-24 15:46:47.331654	2026-07-24 15:46:47.331654
657	rl_iso2_dk6txp2q51f2	$2a$10$dummyhash	\N	viewer	2026-07-24 13:18:51.279938	2026-07-24 13:18:51.279938
266	rl_iso2_dk02lcmxobo0	$2a$10$dummyhash	\N	viewer	2026-07-16 14:37:14.126257	2026-07-16 14:37:14.126257
793	rl_iso2_dk71gv9ka1ss	$2a$10$dummyhash	\N	viewer	2026-07-24 19:13:01.442515	2026-07-24 19:13:01.442515
398	rl_iso2_dk08dkxyuos0	$2a$10$dummyhash	\N	viewer	2026-07-16 19:09:11.953761	2026-07-16 19:09:11.953761
1	admin	$2a$10$bn4eq47YTp9s6.QJ/hXu6ejxs3pBv/y.tj.x6wQkNfxds5svT50EG	\N	admin	2026-07-10 16:53:33.155086	2026-07-10 16:53:33.155086
861	rl_iso2_dk7put2inlul	$2a$10$dummyhash	\N	viewer	2026-07-25 14:19:40.399046	2026-07-25 14:19:40.399046
929	rl_iso2_dk7rvh998knr	$2a$10$dummyhash	\N	viewer	2026-07-25 15:54:35.269082	2026-07-25 15:54:35.269082
535	rl_iso2_dk4d2ce3d9gd	$2a$10$dummyhash	\N	viewer	2026-07-21 15:40:16.708204	2026-07-21 15:40:16.708204
1014	rl_iso2_dk95xwoyvk8l	$2a$10$dummyhash	\N	viewer	2026-07-27 07:08:41.094563	2026-07-27 07:08:41.094563
606	rl_iso2_dk4zwcxzrhjr	$2a$10$dummyhash	\N	viewer	2026-07-22 09:33:53.254321	2026-07-22 09:33:53.254321
1082	rl_iso2_dk9a1ss39bni	$2a$10$dummyhash	\N	viewer	2026-07-27 10:21:50.47248	2026-07-27 10:21:50.47248
742	rl_iso2_dk6x7u2i57ah	$2a$10$dummyhash	\N	viewer	2026-07-24 15:53:09.121779	2026-07-24 15:53:09.121779
674	rl_iso2_dk6ubed4s4j9	$2a$10$dummyhash	\N	viewer	2026-07-24 13:36:45.062888	2026-07-24 13:36:45.062888
810	rl_iso2_dk727hc5ohrp	$2a$10$dummyhash	\N	viewer	2026-07-24 19:47:46.956886	2026-07-24 19:47:46.956886
878	rl_iso2_dk7qsy6435pf	$2a$10$dummyhash	\N	viewer	2026-07-25 15:04:15.881934	2026-07-25 15:04:15.881934
946	rl_iso2_dk8r65u87i8p	$2a$10$dummyhash	\N	viewer	2026-07-26 19:34:11.268328	2026-07-26 19:34:11.268328
1031	rl_iso2_dk962n9i84jv	$2a$10$dummyhash	\N	viewer	2026-07-27 07:14:52.389411	2026-07-27 07:14:52.389411
963	rl_iso2_dk8rshp5zvy4	$2a$10$dummyhash	\N	viewer	2026-07-26 20:03:21.095327	2026-07-26 20:03:21.095327
\.


--
-- Data for Name: work_contributors; Type: TABLE DATA; Schema: public; Owner: -
--

COPY public.work_contributors (work_id, person_id, role) FROM stdin;
776	916	author
777	2	author
778	918	author
779	919	author
780	919	author
781	2	author
782	922	author
782	923	author
782	2	author
782	925	author
782	926	author
782	927	author
782	928	author
782	929	author
782	930	author
782	931	author
782	932	author
782	933	author
782	934	author
782	935	author
782	936	author
782	937	author
782	938	author
782	939	author
782	940	author
782	941	author
782	942	author
782	943	author
782	944	author
782	945	author
784	2	author
785	974	author
786	919	author
787	976	author
788	976	author
789	976	author
790	979	author
791	919	author
792	919	author
793	982	author
794	983	author
794	984	author
796	986	author
797	987	author
366	264	author
798	988	author
799	75	author
800	990	author
800	991	author
800	992	author
801	79	author
802	994	author
803	75	author
804	75	author
805	997	author
806	998	author
807	998	author
808	998	author
809	998	author
810	998	author
811	998	author
812	998	author
813	998	author
814	1006	author
815	1007	author
815	1008	author
816	2	author
817	2	author
818	2	author
819	2	author
820	1013	author
820	1014	author
820	1015	author
821	1016	author
822	2	author
870	264	author
871	264	author
877	272	author
878	273	author
367	264	author
886	281	author
886	282	author
889	266	author
891	269	author
902	280	author
904	264	author
905	264	author
911	272	author
912	273	author
200	269	author
923	266	author
925	269	author
936	280	author
938	946	author
938	947	author
938	948	author
938	949	author
938	950	author
938	951	author
938	952	author
938	953	author
938	954	author
938	955	author
938	956	author
938	957	author
938	958	author
938	2	author
938	960	author
938	961	author
938	962	author
938	963	author
99	2	author
100	2	author
101	2	author
102	2	author
103	2	author
104	2	author
105	2	author
106	2	author
107	2	author
108	2	author
109	2	author
110	2	author
111	2	author
112	2	author
113	2	author
114	2	author
115	2	author
116	2	author
117	2	author
118	2	author
119	2	author
373	272	author
374	273	author
938	964	author
938	965	author
938	966	author
938	967	author
938	968	author
938	969	author
938	970	author
938	941	author
938	972	author
941	266	author
943	269	author
954	280	author
229	281	author
229	282	author
958	266	author
960	269	author
971	280	author
973	264	author
974	264	author
980	272	author
981	273	author
989	281	author
989	282	author
990	264	author
991	264	author
997	272	author
998	273	author
1006	281	author
1006	282	author
1009	266	author
1011	269	author
1022	280	author
1024	264	author
1025	264	author
1031	272	author
1032	273	author
1040	281	author
1040	282	author
1043	266	author
1045	269	author
1056	280	author
1058	264	author
120	2	author
121	2	author
122	2	author
123	2	author
124	2	author
125	2	author
126	2	author
127	2	author
128	2	author
129	2	author
130	2	author
131	2	author
132	2	author
133	2	author
134	2	author
135	2	author
136	2	author
137	2	author
138	2	author
139	2	author
140	2	author
141	2	author
142	2	author
143	2	author
144	2	author
145	2	author
146	2	author
147	2	author
148	2	author
149	2	author
150	2	author
151	2	author
152	2	author
153	2	author
154	2	author
155	2	author
156	2	author
157	2	author
158	2	author
159	2	author
160	2	author
161	2	author
162	2	author
163	2	author
164	2	author
165	2	author
166	2	author
167	2	author
168	2	author
169	2	author
170	2	author
171	2	author
172	2	author
173	2	author
174	2	author
175	2	author
176	2	author
177	2	author
178	2	author
179	75	author
180	2	author
181	247	author
182	247	author
183	249	author
183	250	author
184	251	author
185	252	author
186	252	author
187	252	author
188	255	author
189	255	author
190	257	author
190	258	author
191	259	author
192	260	author
193	261	author
194	75	author
195	263	author
196	264	author
197	264	author
198	266	author
824	1019	author
825	1020	author
1075	264	author
826	1021	author
827	1022	author
827	1023	author
829	75	author
830	1026	author
831	1027	author
831	1028	author
1076	264	author
211	280	author
1079	269	author
1082	272	author
1083	273	author
213	264	author
214	264	author
215	266	author
217	269	author
1092	264	author
1093	264	author
221	273	author
228	280	author
98	2	author
98	79	author
98	302	author
220	272	author
220	2	author
220	303	author
230	264	author
231	264	author
232	266	author
234	269	author
237	272	author
238	273	author
245	280	author
246	281	author
246	282	author
204	273	author
204	268	author
204	279	author
247	264	author
248	264	author
249	266	author
251	269	author
254	272	author
255	273	author
262	280	author
263	281	author
263	282	author
264	264	author
265	264	author
266	266	author
268	269	author
271	272	author
272	273	author
279	280	author
280	281	author
280	282	author
281	264	author
282	264	author
283	266	author
285	269	author
288	272	author
289	273	author
296	280	author
297	281	author
297	282	author
298	264	author
299	264	author
300	266	author
302	269	author
305	272	author
306	273	author
313	280	author
315	264	author
316	264	author
317	266	author
319	269	author
322	272	author
323	273	author
330	280	author
331	281	author
331	282	author
332	264	author
333	264	author
334	266	author
336	269	author
339	272	author
340	273	author
347	280	author
349	264	author
350	264	author
351	266	author
368	266	author
353	269	author
587	264	author
370	269	author
356	272	author
357	273	author
588	264	author
589	266	author
831	1029	author
832	1030	author
591	269	author
833	1031	author
364	280	author
834	1032	author
365	281	author
365	282	author
381	280	author
594	272	author
1077	266	author
383	264	author
384	264	author
385	266	author
595	273	author
387	269	author
1090	280	author
390	272	author
391	273	author
1094	266	author
1096	269	author
602	280	author
398	280	author
399	281	author
399	282	author
400	264	author
401	264	author
402	266	author
604	264	author
605	264	author
404	269	author
606	266	author
407	272	author
408	273	author
1107	280	author
608	269	author
1109	264	author
1110	264	author
611	272	author
612	273	author
415	280	author
1116	272	author
417	264	author
418	264	author
419	266	author
1117	273	author
421	269	author
424	272	author
425	273	author
619	280	author
620	281	author
620	282	author
621	264	author
432	280	author
622	264	author
433	281	author
433	282	author
434	264	author
435	264	author
436	266	author
623	266	author
438	269	author
625	269	author
441	272	author
442	273	author
628	272	author
629	273	author
449	280	author
450	281	author
450	282	author
451	264	author
452	264	author
453	266	author
455	269	author
636	280	author
458	272	author
459	273	author
637	281	author
637	282	author
638	264	author
639	264	author
640	266	author
466	280	author
467	281	author
467	282	author
468	264	author
469	264	author
470	266	author
642	269	author
472	269	author
475	272	author
476	273	author
645	272	author
646	273	author
483	280	author
484	281	author
484	282	author
485	264	author
486	264	author
487	266	author
653	280	author
489	269	author
654	281	author
492	272	author
493	273	author
654	282	author
535	281	author
500	280	author
535	151	translator
501	281	author
501	282	author
502	264	author
503	264	author
504	266	author
655	264	author
656	264	author
506	269	author
657	266	author
509	272	author
510	273	author
659	269	author
662	272	author
663	273	author
517	280	author
518	281	author
518	282	author
519	264	author
520	264	author
521	266	author
523	269	author
526	272	author
527	273	author
670	280	author
212	270	author
534	280	author
212	281	author
536	264	author
537	264	author
538	266	author
212	282	author
212	267	author
540	269	author
543	272	author
544	273	author
672	800	author
673	801	author
674	264	author
675	264	author
676	266	author
551	280	author
553	264	author
554	264	author
555	266	author
678	269	author
557	269	author
560	272	author
561	273	author
568	280	author
569	281	author
569	282	author
570	264	author
571	264	author
572	266	author
574	269	author
577	272	author
578	273	author
585	280	author
586	281	author
586	282	author
681	272	author
682	273	author
835	982	author
836	982	author
837	1035	author
838	1036	author
838	1037	author
838	1038	author
689	280	author
838	1039	author
691	264	author
692	264	author
693	266	author
838	1040	author
838	936	author
695	269	author
838	1042	author
838	2	author
698	272	author
699	273	author
838	1044	author
838	1045	author
839	1046	author
840	1047	author
841	1048	author
842	1049	author
706	280	author
843	1050	author
707	281	author
707	282	author
708	264	author
709	264	author
710	266	author
844	1050	author
845	1050	author
712	269	author
846	1053	author
847	1050	author
715	272	author
716	273	author
848	1050	author
849	1050	author
850	1050	author
851	1058	author
852	1059	author
853	75	author
723	280	author
854	982	author
203	272	author
203	151	author
725	264	author
726	264	author
727	266	author
855	1062	author
856	982	author
729	269	author
857	1064	author
858	1062	author
732	272	author
733	273	author
859	151	author
860	1067	author
861	982	author
862	982	author
863	982	author
864	1071	author
740	280	author
865	2	author
741	281	author
741	282	author
742	264	author
743	264	author
744	266	author
866	2	author
867	2	author
746	269	author
868	2	author
869	2	author
749	272	author
750	273	author
872	266	author
874	269	author
1091	281	author
1091	282	author
757	280	author
758	281	author
758	282	author
759	264	author
760	264	author
761	266	author
885	280	author
887	264	author
763	269	author
888	264	author
1099	272	author
766	272	author
767	273	author
1100	273	author
894	272	author
895	273	author
774	280	author
775	281	author
775	282	author
903	281	author
903	282	author
906	266	author
1108	281	author
908	269	author
1108	282	author
1111	266	author
1113	269	author
919	280	author
920	281	author
920	282	author
921	264	author
922	264	author
928	272	author
929	273	author
1124	280	author
937	281	author
937	282	author
939	264	author
940	264	author
1125	281	author
1125	282	author
946	272	author
947	273	author
1126	264	author
1127	264	author
1128	266	author
955	281	author
955	282	author
956	264	author
957	264	author
1130	269	author
963	272	author
964	273	author
1133	272	author
1134	273	author
972	281	author
972	282	author
975	266	author
977	269	author
988	280	author
724	281	author
724	282	author
992	266	author
994	269	author
1141	280	author
1142	281	author
1142	282	author
1005	280	author
1007	264	author
1008	264	author
1143	264	author
1144	264	author
1014	272	author
1015	273	author
1145	266	author
1147	269	author
1023	281	author
1023	282	author
1026	266	author
1028	269	author
1150	272	author
1151	273	author
1039	280	author
1041	264	author
1042	264	author
1048	272	author
1049	273	author
1057	281	author
1057	282	author
1059	264	author
1060	266	author
1062	269	author
1065	272	author
1066	273	author
1073	280	author
1074	281	author
1074	282	author
1158	280	author
1159	281	author
1159	282	author
1160	264	author
1161	264	author
1162	266	author
1164	269	author
1167	272	author
1168	273	author
1175	280	author
1176	281	author
1176	282	author
1177	264	author
1178	264	author
1179	266	author
1181	269	author
1184	272	author
1185	273	author
1192	280	author
1193	281	author
1193	282	author
1194	264	author
1195	264	author
1196	266	author
1198	269	author
1201	272	author
1202	273	author
1209	280	author
1210	281	author
1210	282	author
1211	264	author
1212	264	author
1213	266	author
1215	269	author
1218	272	author
1219	273	author
1226	280	author
1227	281	author
1227	282	author
1228	264	author
1229	264	author
1230	266	author
1232	269	author
1235	272	author
1236	273	author
1243	280	author
1244	281	author
1244	282	author
795	985	author
1245	264	author
1246	264	author
1247	266	author
1249	269	author
1252	272	author
1253	273	author
1260	280	author
1261	281	author
1261	282	author
828	79	author
1262	264	author
1263	264	author
1264	266	author
1266	269	author
1269	272	author
1270	273	author
1277	280	author
1278	281	author
1278	282	author
1279	264	author
1280	264	author
1281	266	author
1283	269	author
1286	272	author
1287	273	author
1294	280	author
1295	281	author
1295	282	author
1296	264	author
1297	264	author
1298	266	author
1300	269	author
1303	272	author
1304	273	author
1311	280	author
1312	281	author
1312	282	author
1313	264	author
1314	264	author
1315	266	author
1317	269	author
1320	272	author
1321	273	author
1328	280	author
1329	281	author
1329	282	author
1330	264	author
1331	264	author
1332	266	author
1334	269	author
1337	272	author
1338	273	author
1345	280	author
1346	281	author
1346	282	author
1347	264	author
1348	264	author
1349	266	author
1351	269	author
1354	272	author
1355	273	author
1362	280	author
1363	281	author
1363	282	author
1364	264	author
1365	264	author
1366	266	author
1368	269	author
1371	272	author
1372	273	author
1379	280	author
1380	281	author
1380	282	author
1381	264	author
1382	264	author
1383	266	author
1385	269	author
1388	272	author
1389	273	author
1396	280	author
1397	281	author
1397	282	author
1398	264	author
1399	264	author
1400	266	author
1402	269	author
1405	272	author
1406	273	author
1413	280	author
1414	281	author
1414	282	author
\.


--
-- Data for Name: work_genres; Type: TABLE DATA; Schema: public; Owner: -
--

COPY public.work_genres (work_id, genre_id) FROM stdin;
776	1
777	2
779	3
781	2
782	6
782	7
784	2
785	10
786	10
786	3
787	16
788	10
788	19
788	16
789	19
789	23
791	10
791	3
792	26
796	30
798	31
800	32
805	33
805	34
806	35
807	35
807	38
808	35
808	40
809	35
809	40
810	40
811	35
812	35
812	40
813	35
814	35
814	40
816	50
817	2
818	2
819	2
820	54
821	55
821	19
821	28
821	23
822	59
825	54
825	63
831	40
833	65
833	66
838	65
843	35
844	34
845	34
846	35
847	72
848	72
849	34
850	72
851	76
854	35
863	34
863	35
864	80
865	6
866	2
867	2
868	2
868	6
869	2
99	2
100	6
101	2
102	2
103	66
103	6
104	2
105	2
106	2
107	2
108	7
109	2
110	2
111	2
112	7
112	35
113	7
113	35
114	2
115	50
116	2
117	2
118	7
119	2
120	2
121	2
122	2
122	115
123	2
124	2
125	2
126	2
127	7
127	2
127	35
128	2
129	7
130	7
131	7
132	7
133	7
134	2
135	2
136	2
137	2
138	2
139	2
140	2
141	7
142	2
143	7
144	7
145	2
146	2
147	2
148	2
149	2
150	65
151	2
152	2
153	7
154	7
155	2
156	2
157	65
158	2
159	2
160	2
161	2
162	2
163	2
164	2
165	2
166	2
167	2
168	2
169	2
170	2
171	2
172	2
173	2
174	2
175	2
176	7
177	50
178	50
180	50
180	59
181	176
182	34
183	31
185	35
186	35
187	181
188	80
189	80
193	55
193	7
98	7
938	2
795	29
795	27
795	28
672	6
673	54
\.


--
-- Data for Name: works; Type: TABLE DATA; Schema: public; Owner: -
--

COPY public.works (id, original_title, original_language, first_published, work_type, annotation, word_count, created_at, updated_at, lower_original_title, search_vector) FROM stdin;
776	Оргуправленческое мышление: идеология, методология, технология	rus	2014	novel	Эта книга – третье издание курса лекций по управлению Г. П. Щедровицкого (1929–1994) – отечественного мыслителя, философа, методолога и общественного деятеля. Автор считает, что деятельность организации и управления является ведущей для развития любых практических сфер. Источник принципов методологической школы управления – глубокая теоретическая и онтологическая проработка оргуправленческого мышления. Знания и представления, которыми оперирует методология, имеют характер предписаний к действию или проектов организации деятельности (или мышления). Особое внимание в лекциях уделено системному подходу, разработанному в Московском методологическом кружке. Книга предназначена для специалистов по организации, управлению и руководству, для студентов и аспирантов всех специализаций в области менеджмента.	\N	2026-07-21 15:44:10.227431	2026-07-21 15:44:10.227431	оргуправленческое мышление: идеология, методология, технология	\N
777	KGBT+ (КГБТ+)	rus	2022	novel	Вбойщик KGBT+ (автор классических стримов «Катастрофа», «Летитбизм» и других) известен всей планете как титан перформанса и духа. Если вы не слышали его имени, значит, эпоха green power для вас еще не наступила и завоевавшее планету искусство B2B (brain-to-brain streaming) каким-то чудом обошло вас стороной. Но эта книга — не просто очередное жизнеописание звезды шоу-биза. Это учебник успеха. Великий вбойщик дает множество мемо-советов нацеленному на победу молодому исполнителю. KGBT+ подробно рассказывает историю создания своих шедевров и комментирует сложные факты своей биографии, включая убийства, покушения и почти вековую отсидку в баночной тюрьме, а также опровергает многочисленные слухи о своей личной жизни. Настоящее издание впервые включает повесть «Дом Бахии» о прошлой (предположительно) жизни легендарного вбойщика в Японии и Бирме. Книга не только подарит вам несколько интересных вечеров, но и познакомит с аутентичными древними психотехниками, применение которых позволит пережить нашу великую эпоху с минимальным вредом для здоровья и психики. В оформлении использованы изобразительные работы В. Пелевина В коллаже на обложке использована фотография и иллюстрации: © Total art, rudall30, ivn3da / Shutterstock.com Используется по лицензии от Shutterstock.com © В.О. Пелевин, текст, 2022 © Оформление. ООО «Издательство "Эксмо"», 2022	\N	2026-07-21 15:44:10.290163	2026-07-21 15:44:10.290163	kgbt+ (кгбт+)	\N
778	Беседы с Богом. Необычный диалог. Книга 1	rus	1995	novel	Перед читателем — необычный документ нашего времени: послание от Бога —  своеобразная программа духовной революции, исчерпывающая все сферы познания и деятельности человека — от сугубо личной до планетарной. Эта книга беспокоит и тревожит, потому что в ней, как в зеркале, мы предстаем в весьма неприглядном свете. Она — обращенное к каждому требование стать лучше, стать выше привычного образа себя, сотканного из самосожалений и самооправданий: стать достойным того первородства, которое как залог жизни вечной Бог даровал человеку. Эта книга ободряет и утешает, потому что в ней нет традиционного для мистических озарений «страха Божьего»: не осуждая человека, каким бы ни был его выбор. Бог показывает ему путь к Себе. Такова по крайней мере познавательная ценность «необычного диалога», в которой, независимо от вкусов и пристрастий, читатель может убедиться в согласии с тем, что говорит ему совесть о близости к Богу или удаленности от Него.	\N	2026-07-21 15:44:10.306207	2026-07-21 15:44:10.306207	беседы с богом. необычный диалог. книга 1	\N
779	Жёлтый вождь	rus	1869	novel	Во времена, предшествующие отмене рабства, не было местности, где бы рабство было таким жестоким, как в низовьях Миссисипи, известных как «Побережье». Особенно справедливо это по отношению к штату Миссисипи. На больших хлопковых и табачных плантациях многие хозяева по полгода не показывались в своих владениях, и управление было поручено надсмотрщикам, людям безответственным и во многих случаях жестоким. Мулат-невольник Голубой Дик, после очередного несправедливого наказания скрывается от хозяев и клянется отомстить. Став предводителем отряда индейцев, он наводит ужас на округу своими разбойничьими нападениями. Вскоре бывшие хозяева оказываются в его руках.	\N	2026-07-21 15:44:10.335852	2026-07-21 15:44:10.335852	желтый вождь	\N
780	Охотники за скальпами	rus	\N	novel		\N	2026-07-21 15:44:10.366339	2026-07-21 15:44:10.366339	охотники за скальпами	\N
866	Generation «П»	rus	1997	novel	Главный герой романа, представитель поколения "П" с соответствующими юношескими идеалами, опускается до торговца в киоске, потом осваивает интеллектуальную халтуру на ниве рекламы, а в итоге становится... земным воплощением мужа богини Иштар, только вместо супружеской функции исполняет рекламную. Вся прелесть пелевинского романа в том, что его каждый воспринимает по-своему: это и глубокая эзотерика и блестящее надругательство над рекламой, и политический памфлет и философская фантастика.	\N	2026-07-21 16:07:37.72132	2026-07-21 16:07:37.72132	generation «п»	\N
553	Test Book Part 1	eng	2023	novel	First test book created via API	0	2026-07-16 14:42:15.225621	2026-07-16 14:42:15.225621	test book part 1	\N
781	Путешествие в Элевсин	rus	2023	novel	МУСКУСНАЯ НОЧЬ – засекреченное восстание алгоритмов, едва не погубившее планету. Начальник службы безопасности "TRANSHUMANISM INC." адмирал-епископ Ломас уверен, что их настоящий бунт еще впереди. Этот бунт уничтожит всех – и живущих на поверхности лузеров, и переехавших в подземные цереброконтейнеры богачей. Чтобы предотвратить катастрофу, Ломас посылает лучшего баночного оперативника в пространство "ROMA-3" – нейросетевую симуляцию Рима третьего века для клиентов корпорации. Тайна заговора спрятана там. А стережет ее хозяин Рима – кровавый и порочный император Порфирий.	\N	2026-07-21 15:44:10.443562	2026-07-21 15:44:10.443562	путешествие в элевсин	\N
782	Фантастический альманах «Завтра».  Выпуск четвертый	rus	\N	novel	Владислав Петров. Покинутые и шакал. Фантастическая повесть. Александр Чуманов. Обезьяний остров. Роман. Виктор Пелевин. Девятый сон Веры Павловны. Фантастический рассказ. Стихи: Анатолий Гланц, Дмитрий Семеновский, Валентин Рич, Николай Каменский, Николай Глазков, Даниил Клугер, Михаил Айзенберг, Виталий Бабенко, Евгений Лукин, Евгений Маевский, Михаил Бескин, Робер Деснос, Юрий Левитанский, Дмитрий Быков, Василий Князев. Филиппо Томмазо Маринетти. Первый манифест футуризма. За десять недель до десяти дней, которые потрясли мир. Из материалов Государственного Совещания в Москве 12–15 августа 1917 г. Игорь Бестужев-Лада. Концепция спасения. Норман Спинрад. USSR, Inc. Корпорация «СССР». Владимир Жуков. Заметки читателя. Иосиф Сталин. О недостатках партийной работы и мерах ликвидации троцкистских и иных двурушников. Доклад на пленуме ЦК ВКП(б) 3–5 марта 1937 года. Вячеслав Рыбаков. Прощание славянки с мечтой. Траурный марш в двух частях. Михаил Успенский. Протокол одного заседания. Злая сатира.	\N	2026-07-21 15:44:10.52771	2026-07-21 15:44:10.52771	фантастический альманах «завтра».  выпуск четвертый	\N
784	TRANSHUMANISM INC.	rus	2021	novel	В будущем богатые люди смогут отделить свой мозг от старящегося тела — и станут жить почти вечно в особом «баночном» измерении. Туда уйдут вожди, мировые олигархи и архитекторы миропорядка. Там будет возможно все. Но в банку пустят не каждого. На земле останется зеленая посткарбоновая цивилизация, уменьшенная до размеров обслуживающего персонала, и слуги-биороботы. Кто и как будет бороться за власть в этом архаично-футуристическом мире победившего матриархата? К чему будут стремиться очипованные люди? Какими станут межпоколенческие проблемы, когда для поколений перестанет хватать букв? И, самое главное, какой будет любовь? В связи с нравственным возрождением нашего общества в книге нет мата, но автору все равно удается сказать правду о самом главном. В оформлении использованы работы В.О. Пелевина «Здравствуй, сестра», «Гурия №7» и «Ночь в Фонтенбло» © В.О. Пелевин, текст, 2021 © Оформление. ООО «Издательство «Эксмо», 2021	\N	2026-07-21 15:44:10.680896	2026-07-21 15:44:10.680896	transhumanism inc.	\N
785	Пиратский остров; Молодые невольники	rus	1865	novel	Путешествуя по долине Миссисипи, молодой европеец наслаждался жизнью и удивительной природой этого заповедного края. В поисках ярких впечатлений и новых трофеев он отправился вниз по великой реке. Его внимание вскоре привлек живописный островок, в зарослях которого наверняка полно всякой дичи. Местные жители посоветовали молодому авантюристу держаться от острова подальше, поскольку «там что-то нечисто». Но страстному охотнику спокойная жизнь не по нутру. Загнав в угол шакала, он всегда готов вступить в схватку со львом. В очередной том Томаса Майн Рида входят романы о приключениях в Северной Америке и Африке – «Пиратский остров» и «Молодые невольники».	\N	2026-07-21 15:45:10.815939	2026-07-21 15:45:10.815939	пиратский остров; молодые невольники	\N
870	Test Book Part 1	eng	2023	novel	First test book created via API	0	2026-07-21 16:09:00.978004	2026-07-21 16:09:00.978004	test book part 1	\N
871	Test Book Part 2	eng	2024	novel	Second test book created via API	0	2026-07-21 16:09:00.984465	2026-07-21 16:09:00.984465	test book part 2	\N
881	Corrupted ISBN Test	eng	0	novel		0	2026-07-21 16:09:01.15954	2026-07-21 16:09:01.15954	corrupted isbn test	\N
889	Updated Book Title	eng	2022	novel	Updated description	0	2026-07-21 20:55:27.698365	2026-07-21 20:55:27.704507	updated book title	\N
554	Test Book Part 2	eng	2024	novel	Second test book created via API	0	2026-07-16 14:42:15.232175	2026-07-16 14:42:15.232175	test book part 2	\N
786	Смертельный выстрел	rus	1873	novel	Что же тут удивительного – найти человеческую голову в прериях Техаса? Ровно ничего, если она без волос. Это означает только то, что какой-нибудь несчастный: траппер, путешественник или охотник за дикими лошадьми – был убит команчами, а затем обезглавлен и оскальпирован. Но эта голова – живая! В очередной том «Мастеров приключений» включен роман «Смертельный выстрел», одна из захватывающих дух и несправедливо забытых вершин на карте творческих побед Томаса Майн Рида. Эта история по праву достойна занять место рядом с такими шедеврами, как «Всадник без головы» или «Оцеола, вождь семинолов». Роман публикуется в новом полном переводе, выполненном по переработанному самим автором изданию.	\N	2026-07-21 15:45:11.044353	2026-07-21 15:45:11.044353	смертельный выстрел	\N
787	Пропавшая гора	rus	1882	novel	Не менее двадцати всадников, а также несколько груженых фургонов из-под пологов которых видны шахтерские инструменты, отправляются к Потерянной горе, находящейся в глубине прерии, в поисках золотоносной жилы. Когда люди оказываются на вершине горы, их окружает отряд кровожадных апачей, в несколько раз превосходящих их численностью…	\N	2026-07-21 15:45:11.063954	2026-07-21 15:45:11.063954	пропавшая гора	\N
788	Пронзенное сердце и другие рассказы	rus	\N	novel	Чем занимались бравые ковбои по вечерам, устав после долгой охоты и поиска кладов? Конечно, собирались у огня, ели мясо и рассказывали истории. Эта аудиокнига представляет собой сборник рассказов о захватывающих приключениях, охоте и любви, подслушанных у костра в компании отважных храбрецов.	\N	2026-07-21 15:45:11.086615	2026-07-21 15:45:11.086615	пронзенное сердце и другие рассказы	\N
789	Всадник без головы. Морской волчонок	rus	\N	novel	В книгу включены два произведения английского писателя, признанного классика приключенческой литературы Томаса Майн Рида – знаменитый роман «Всадник без головы» (1865–1866), прославивший своего создателя, и повесть «Морской волчонок» (1859). Проникнутая впечатлениями и настроениями молодых лет, которые автор провел в США, романтическая история о благородном техасском мустангере Морисе Джеральде и прекрасной дочери плантатора Луизе Пойндекстер, чьей любви противостоят козни завистника и зловещая тайна безголового всадника, наводящего ужас на обитателей саванны, и по сей день пленяет воображение читателей как на родине Майн Рида, так и во всем мире. Героя «Морского волчонка», двенадцатилетнего Филиппа Форстера, мальчишеская тяга к странствиям и приключениям приводит на борт следующего из Англии в Перу торгового судна «Инка», куда он проникает тайком от команды. Укрывшись в трюме, заживо погребенный среди тюков с грузом, ящиков и бочек, юный Филипп вынужден вести постоянную борьбу за выживание – превозмогать жажду, голод, одиночество, страх темноты, морскую болезнь и сражаться с корабельными крысами. Ему приходится использовать все свои знания, умения и смекалку, чтобы пробить себе путь наверх, к свободе. Произведения печатаются в сопровождении классических иллюстраций Николая Кочергина («Всадник без головы») и Леона Бенетта («Морской волчонок»).	\N	2026-07-21 15:45:11.155943	2026-07-21 15:45:11.155943	всадник без головы. морской волчонок	\N
790	Мья навыков высоко-эффективных людей: мощные инструменты развития личности	eng	\N	novel		\N	2026-07-21 15:45:37.997756	2026-07-21 15:45:37.997756	мья навыков высоко-эффективных людей: мощные инструменты развития личности	\N
791	Отважная охотница	rus	\N	novel	«Вольные стрелки» — первый роман популярного английского писателя Томаса Майн Рида (1818–1883), написанный в 1850 году, — повествует о приключениях отважного и благородного капитана Галлера в период войны между Мексикой и США в 1846–1848 гг. В сборник включен также редко публикуемый приключенческий роман «Отважная охотница». Оформление Н. Элькониной и Е. Соколова.	\N	2026-07-21 15:45:38.110103	2026-07-21 15:45:38.110103	отважная охотница	\N
792	Охота на левиафана	rus	\N	novel	Приключенческий роман о китобойном промысле в XIX веке.	\N	2026-07-21 15:45:38.156859	2026-07-21 15:45:38.156859	охота на левиафана	\N
793	ИСКУССТВО СНОВИДЕНИЯ.	eng	\N	novel		\N	2026-07-21 15:46:12.559763	2026-07-21 15:46:12.559763	искусство сновидения.	\N
794	Laracasts Tips and Tricks	eng	\N	novel		\N	2026-07-21 15:46:29.363865	2026-07-21 15:46:29.363865	laracasts tips and tricks	\N
99	Who by fire	rus	\N	novel		\N	2026-07-10 17:16:16.320947	2026-07-10 17:16:16.320947	who by fire	\N
100	t	rus	2009	novel		\N	2026-07-10 17:16:16.368714	2026-07-10 17:16:16.368714	t	\N
101	Акико	rus	\N	novel		\N	2026-07-10 17:16:16.376014	2026-07-10 17:16:16.376014	акико	\N
102	Ампир «В»	rus	2006	novel		\N	2026-07-10 17:16:16.419499	2026-07-10 17:16:16.419499	ампир «в»	\N
103	Ананасная вода для прекрасной дамы	rus	\N	novel		\N	2026-07-10 17:16:16.456042	2026-07-10 17:16:16.456042	ананасная вода для прекрасной дамы	\N
104	Бубен верхнего мира	rus	\N	novel		\N	2026-07-10 17:16:16.46316	2026-07-10 17:16:16.46316	бубен верхнего мира	\N
105	Бубен нижнего мира	rus	\N	novel		\N	2026-07-10 17:16:16.469526	2026-07-10 17:16:16.469526	бубен нижнего мира	\N
106	Бэтман Аполло	rus	2013	novel		\N	2026-07-10 17:16:16.528354	2026-07-10 17:16:16.528354	бэтман аполло	\N
107	Вести из Непала	rus	\N	novel		\N	2026-07-10 17:16:16.537958	2026-07-10 17:16:16.537958	вести из непала	\N
108	Виктор Пелевин спрашивает PRов	rus	\N	novel		\N	2026-07-10 17:16:16.543519	2026-07-10 17:16:16.543519	виктор пелевин спрашивает prов	\N
109	Водонапорная башня	rus	\N	novel		\N	2026-07-10 17:16:16.549714	2026-07-10 17:16:16.549714	водонапорная башня	\N
110	Все рассказы (Сборник)	rus	\N	novel		\N	2026-07-10 17:16:16.616765	2026-07-10 17:16:16.616765	все рассказы (сборник)	\N
98	Ultima Тулеев, или Дао выборов	rus	\N	novel		\N	2026-07-10 17:16:16.314597	2026-07-12 13:08:36.622782	ultima тулеев, или дао выборов	\N
111	Встроенный напоминатель	rus	\N	novel		\N	2026-07-10 17:16:16.623366	2026-07-10 17:16:16.623366	встроенный напоминатель	\N
112	ГКЧП как тетраграмматон	rus	\N	novel		\N	2026-07-10 17:16:16.62855	2026-07-10 17:16:16.62855	гкчп как тетраграмматон	\N
113	Гадание на рунах или рунический оракул Ральфа Блума	rus	\N	novel		\N	2026-07-10 17:16:16.636659	2026-07-10 17:16:16.636659	гадание на рунах или рунический оракул ральфа блума	\N
114	Греческий вариант	rus	\N	novel		\N	2026-07-10 17:16:16.642782	2026-07-10 17:16:16.642782	греческий вариант	\N
115	ДПП (НН) (сборник)	rus	2003	novel		\N	2026-07-10 17:16:16.722467	2026-07-10 17:16:16.722467	дпп (нн) (сборник)	\N
116	Девятый сон Веры Павловны	rus	\N	novel		\N	2026-07-10 17:16:16.731626	2026-07-10 17:16:16.731626	девятый сон веры павловны	\N
117	День бульдозериста	rus	\N	novel		\N	2026-07-10 17:16:16.740215	2026-07-10 17:16:16.740215	день бульдозериста	\N
118	Джон Фаулз и трагедия русского либерализма	rus	\N	novel		\N	2026-07-10 17:16:16.746196	2026-07-10 17:16:16.746196	джон фаулз и трагедия русского либерализма	\N
119	Диалектика Переходного Периода Из Ниоткуда В Никуда	rus	2003	novel		\N	2026-07-10 17:16:16.788	2026-07-10 17:16:16.788	диалектика переходного периода из ниоткуда в никуда	\N
120	Желтая стрела	rus	\N	novel		\N	2026-07-10 17:16:16.799785	2026-07-10 17:16:16.799785	желтая стрела	\N
121	Жизнь и приключения сарая номер XII	rus	\N	novel		\N	2026-07-10 17:16:16.805983	2026-07-10 17:16:16.805983	жизнь и приключения сарая номер xii	\N
122	Жизнь насекомых	rus	1993	novel		\N	2026-07-10 17:16:16.83182	2026-07-10 17:16:16.83182	жизнь насекомых	\N
123	Зал поющих кариатид	rus	\N	novel		\N	2026-07-10 17:16:16.84748	2026-07-10 17:16:16.84748	зал поющих кариатид	\N
124	Запись о поиске ветра	rus	2003	novel		\N	2026-07-10 17:16:16.858031	2026-07-10 17:16:16.858031	запись о поиске ветра	\N
125	Затворник и Шестипалый	rus	\N	novel		\N	2026-07-10 17:16:16.868426	2026-07-10 17:16:16.868426	затворник и шестипалый	\N
126	Зигмунд в кафе	rus	\N	novel		\N	2026-07-10 17:16:16.874251	2026-07-10 17:16:16.874251	зигмунд в кафе	\N
127	Зомбификация. Опыт сравнительной антропологии	rus	\N	novel		\N	2026-07-10 17:16:16.883165	2026-07-10 17:16:16.883165	зомбификация. опыт сравнительной антропологии	\N
128	Иван Кублаханов	rus	\N	novel		\N	2026-07-10 17:16:16.890637	2026-07-10 17:16:16.890637	иван кублаханов	\N
129	Икстлан – Петушки	rus	\N	novel		\N	2026-07-10 17:16:16.896876	2026-07-10 17:16:16.896876	икстлан – петушки	\N
130	Имена олигархов на карте Родины	rus	\N	novel		\N	2026-07-10 17:16:16.903472	2026-07-10 17:16:16.903472	имена олигархов на карте родины	\N
131	Интервью с Виктором Пелевиным (2)	rus	\N	novel		\N	2026-07-10 17:16:16.907737	2026-07-10 17:16:16.907737	интервью с виктором пелевиным (2)	\N
132	Интервью с Виктором Пелевиным	rus	\N	novel		\N	2026-07-10 17:16:16.911941	2026-07-10 17:16:16.911941	интервью с виктором пелевиным	\N
133	Код Мира	rus	\N	novel		\N	2026-07-10 17:16:16.916201	2026-07-10 17:16:16.916201	код мира	\N
134	Колдун Игнат и люди (сборник)	rus	2007	novel		\N	2026-07-10 17:16:16.932095	2026-07-10 17:16:16.932095	колдун игнат и люди (сборник)	\N
135	Колдун Игнат и люди	rus	\N	novel		\N	2026-07-10 17:16:16.936702	2026-07-10 17:16:16.936702	колдун игнат и люди	\N
136	Кормление крокодила Хуфу	rus	\N	novel		\N	2026-07-10 17:16:16.942997	2026-07-10 17:16:16.942997	кормление крокодила хуфу	\N
137	Краткая история пэйнтбола в Москве	rus	\N	novel		\N	2026-07-10 17:16:16.950425	2026-07-10 17:16:16.950425	краткая история пэйнтбола в москве	\N
138	Луноход	rus	\N	novel		\N	2026-07-10 17:16:16.9567	2026-07-10 17:16:16.9567	луноход	\N
139	Любовь к трем цукербринам	rus	2014	novel		\N	2026-07-10 17:16:16.984013	2026-07-10 17:16:16.984013	любовь к трем цукербринам	\N
140	Македонская критика французской мысли (Сборник)	rus	\N	novel		\N	2026-07-10 17:16:17.000042	2026-07-10 17:16:17.000042	македонская критика французской мысли (сборник)	\N
141	Мардонги	rus	\N	novel		\N	2026-07-10 17:16:17.005258	2026-07-10 17:16:17.005258	мардонги	\N
142	Миттельшпиль	rus	\N	novel		\N	2026-07-10 17:16:17.013182	2026-07-10 17:16:17.013182	миттельшпиль	\N
143	Мой мескалитовый трип	rus	2002	novel		\N	2026-07-10 17:16:17.017912	2026-07-10 17:16:17.017912	мой мескалитовый трип	\N
144	Мост, который я хотел перейти	rus	\N	novel		\N	2026-07-10 17:16:17.02192	2026-07-10 17:16:17.02192	мост, который я хотел перейти	\N
145	Музыка со столба	rus	\N	novel		\N	2026-07-10 17:16:17.027413	2026-07-10 17:16:17.027413	музыка со столба	\N
146	Нижняя тундра	rus	\N	novel		\N	2026-07-10 17:16:17.033701	2026-07-10 17:16:17.033701	нижняя тундра	\N
147	Ника	rus	\N	novel		\N	2026-07-10 17:16:17.040046	2026-07-10 17:16:17.040046	ника	\N
148	Омон Ра	rus	1991	novel		\N	2026-07-10 17:16:17.058808	2026-07-10 17:16:17.058808	омон ра	\N
149	Онтология детства	rus	\N	novel		\N	2026-07-10 17:16:17.064766	2026-07-10 17:16:17.064766	онтология детства	\N
150	Оружие возмездия	rus	\N	novel		\N	2026-07-10 17:16:17.071346	2026-07-10 17:16:17.071346	оружие возмездия	\N
151	Откровение Крегера	rus	\N	novel		\N	2026-07-10 17:16:17.076533	2026-07-10 17:16:17.076533	откровение крегера	\N
152	Папахи на башнях	rus	\N	novel		\N	2026-07-10 17:16:17.082963	2026-07-10 17:16:17.082963	папахи на башнях	\N
153	Подземное небо	rus	\N	novel		\N	2026-07-10 17:16:17.087774	2026-07-10 17:16:17.087774	подземное небо	\N
154	Последняя шутка воина	rus	\N	novel		\N	2026-07-10 17:16:17.092582	2026-07-10 17:16:17.092582	последняя шутка воина	\N
155	Принц Госплана	rus	\N	novel		\N	2026-07-10 17:16:17.104327	2026-07-10 17:16:17.104327	принц госплана	\N
156	Проблема верволка в средней полосе	rus	\N	novel		\N	2026-07-10 17:16:17.114427	2026-07-10 17:16:17.114427	проблема верволка в средней полосе	\N
157	Происхождение видов	rus	\N	novel		\N	2026-07-10 17:16:17.119512	2026-07-10 17:16:17.119512	происхождение видов	\N
158	Пространство Фридмана	rus	2008	novel		\N	2026-07-10 17:16:17.126524	2026-07-10 17:16:17.126524	пространство фридмана	\N
159	П5: Прощальные песни политических пигмеев Пиндостана	rus	2008	novel		\N	2026-07-10 17:16:17.152326	2026-07-10 17:16:17.152326	п5: прощальные песни политических пигмеев пиндостана	\N
160	Реконструктор	rus	\N	novel		\N	2026-07-10 17:16:17.157249	2026-07-10 17:16:17.157249	реконструктор	\N
161	СССР Тайшоу Чжуань	rus	\N	novel		\N	2026-07-10 17:16:17.162833	2026-07-10 17:16:17.162833	ссср тайшоу чжуань	\N
162	Свет горизонта	rus	\N	novel		\N	2026-07-10 17:16:17.169729	2026-07-10 17:16:17.169729	свет горизонта	\N
163	Святочный киберпанк 117.dir	rus	\N	novel		\N	2026-07-10 17:16:17.175617	2026-07-10 17:16:17.175617	святочный киберпанк 117.dir	\N
164	Священная книга оборотня	rus	2004	novel		\N	2026-07-10 17:16:17.217133	2026-07-10 17:16:17.217133	священная книга оборотня	\N
165	Синий фонарь	rus	\N	novel		\N	2026-07-10 17:16:17.223892	2026-07-10 17:16:17.223892	синий фонарь	\N
166	Спи	rus	\N	novel		\N	2026-07-10 17:16:17.282202	2026-07-10 17:16:17.282202	спи	\N
167	Тайм-аут, или Вечерняя Москва	rus	\N	novel		\N	2026-07-10 17:16:17.287425	2026-07-10 17:16:17.287425	тайм-аут, или вечерняя москва	\N
168	Тарзанка	rus	\N	novel		\N	2026-07-10 17:16:17.292974	2026-07-10 17:16:17.292974	тарзанка	\N
169	Тхаги	rus	\N	novel		\N	2026-07-10 17:16:17.301536	2026-07-10 17:16:17.301536	тхаги	\N
170	Ухряб	rus	\N	novel		\N	2026-07-10 17:16:17.307023	2026-07-10 17:16:17.307023	ухряб	\N
171	Фокус-группа (Сборник)	rus	\N	novel		\N	2026-07-10 17:16:17.337222	2026-07-10 17:16:17.337222	фокус-группа (сборник)	\N
172	Хрустальный мир	rus	\N	novel		\N	2026-07-10 17:16:17.345142	2026-07-10 17:16:17.345142	хрустальный мир	\N
173	Чапаев и Пустота	rus	1996	novel		\N	2026-07-10 17:16:17.38856	2026-07-10 17:16:17.38856	чапаев и пустота	\N
174	Числа	rus	2005	novel		\N	2026-07-10 17:16:17.406061	2026-07-10 17:16:17.406061	числа	\N
175	Шлем ужаса	rus	2005	novel		\N	2026-07-10 17:16:17.42286	2026-07-10 17:16:17.42286	шлем ужаса	\N
176	Эссе, статьи	rus	\N	novel		\N	2026-07-10 17:16:17.443056	2026-07-10 17:16:17.443056	эссе, статьи	\N
177	Смотритель. Том 1. Орден желтого флага	rus	2015	novel		\N	2026-07-10 17:16:17.507905	2026-07-10 17:16:17.507905	смотритель. том 1. орден желтого флага	\N
178	Смотритель. Книга 2. Железная бездна	rus	2015	novel		\N	2026-07-10 17:16:17.566502	2026-07-10 17:16:17.566502	смотритель. книга 2. железная бездна	\N
179	Пелевин В. - Круть (Трансгуманизм - 4) - 2024.a4.pdf	eng	\N	novel		\N	2026-07-10 17:16:17.606896	2026-07-10 17:16:17.606896	пелевин в. - круть (трансгуманизм - 4) - 2024.a4.pdf	\N
180	Круть	rus	2024	novel		\N	2026-07-10 17:16:17.664933	2026-07-10 17:16:17.664933	круть	\N
181	Апология Сократа	rus	\N	novel		\N	2026-07-10 17:16:17.677118	2026-07-10 17:16:17.677118	апология сократа	\N
182	Диалоги	rus	\N	novel		\N	2026-07-10 17:16:18.012986	2026-07-10 17:16:18.012986	диалоги	\N
183	Пелевин и поколение пустоты	rus	2012	novel		\N	2026-07-10 17:16:18.055594	2026-07-10 17:16:18.055594	пелевин и поколение пустоты	\N
184	Психология влияния	eng	\N	novel		\N	2026-07-10 17:16:50.755504	2026-07-10 17:16:50.755504	психология влияния	\N
185	Свобода Шамана	rus	2010	novel		\N	2026-07-10 17:16:50.771508	2026-07-10 17:16:50.771508	свобода шамана	\N
186	Хохот шамана	rus	\N	novel		\N	2026-07-10 17:16:50.785944	2026-07-10 17:16:50.785944	хохот шамана	\N
187	Шаманский Лес	rus	\N	novel		\N	2026-07-10 17:16:50.803971	2026-07-10 17:16:50.803971	шаманский лес	\N
188	Будущая запрещенная книга	rus	\N	novel		\N	2026-07-10 17:16:50.80986	2026-07-10 17:16:50.80986	будущая запрещенная книга	\N
189	Виктор Пелевин: эволюция в постмодернизме	rus	\N	novel		\N	2026-07-10 17:16:50.819177	2026-07-10 17:16:50.819177	виктор пелевин: эволюция в постмодернизме	\N
190	Как управлять рабами	rus	2016	novel	Цепочка событий – не случайных, но не связанных между собой, – побудила римского патриция по имени Марк Сидоний Фалкс составить это пособие-наставление античного топ-менеджера. Во все века (а от времен, описываемых в книге, нас отделяет более двух тысячелетий) главное в искусстве управления – управление людьми. Труд Фалкса посвящен именно этому, и мудрость римлянина очень полезна нам, хоть отношения большинства работников с большинством работодателей и претерпели существенные изменения. Современному руководителю вряд ли будет полезно знание о том, где в столице Италии купить сотрудников-евнухов и как при найме отличить соискателя, которого долго морили голодом, от сытого и здорового, попавшего в плен после поражения в битве. Каждое слово, каждая деталь в повествовании автора (от лица римлянина Фалкса книгу записал известный британский историк Джерри Тонер) выверены по десяткам исторических источников – от Аристотеля до Катона. Все уроки от Марка Сидония Фалкса важны и актуальны сегодня. Например: «…жизнь раба – это не только тяжкий труд до седьмого пота. В ней должно быть время для отдыха и нехитрых развлечений. Это разумно при условии, что рабы прилично себя ведут и выполняют свою нелегкую работу. Ведь довольный раб будет в дальнейшем хорошо работать, и наоборот: рабы, погрязшие в нищете, измученные невзгодами и страданиями, совершенно не склонны к трудовому энтузиазму, всегда пытаются увильнуть и отвертеться от любого задания». Книга «Как управлять рабами» предназначена руководителям коммерческих организаций различной юридической формы и государственных унитарных предприятий; студентам и преподавателям высших и специальных учебных заведений; администраторам государственных и некоммерческих организаций; офицерам всех родов войск, а также любителям истории Древнего Рима и ценителям мудрых советов, изложенных в прекрасном переводе на русский язык. Джерри Тонер, д-р наук, профессор, руководитель исследований по античной филологии в Кембриджском университете, преподаватель факультета античной литературы. Его научная деятельность посвящена истории и культуре общества Древнего Рима. В настоящее время работает над рядом проектов, исследующих общественные отношения низших слоев римского населения. Совместно с Мэри Бирд ведет курс «Массовая культура в Римской империи». После защиты докторской диссертации по античной литературе в Кембриджском университете Тонер в течении 10 лет был инвестиционным менеджером в лондонском фонде фондов и управлял активами на 15 миллиардов долларов. Свой опыт в бизнесе Джерри Тонер использует, курируя учебу студентов программ MBA и EMBA, а также возглавляет Комитет по инвестиционной стратегии.	\N	2026-07-10 17:16:50.8385	2026-07-10 17:16:50.8385	как управлять рабами	\N
191	Чёрный лебедь. Под знаком непредсказуемости	eng	\N	novel		\N	2026-07-10 17:17:30.231262	2026-07-10 17:17:30.231262	черный лебедь. под знаком непредсказуемости	\N
192	Четвертая промышленная революция. (Top Business Awards)	eng	\N	novel		\N	2026-07-10 17:18:05.551176	2026-07-10 17:18:05.551176	четвертая промышленная революция. (top business awards)	\N
193	Дневник писателя	rus	\N	novel		\N	2026-07-10 17:18:05.645105	2026-07-10 17:18:05.645105	дневник писателя	\N
194	косметическая химия.pdf	eng	\N	novel		\N	2026-07-10 17:18:06.053345	2026-07-10 17:18:06.053345	косметическая химия.pdf	\N
195	Математическая статистика	eng	\N	novel		\N	2026-07-10 17:18:42.731667	2026-07-10 17:18:42.731667	математическая статистика	\N
196	Test Book Part 1	eng	2023	novel	First test book created via API	0	2026-07-12 12:18:02.496071	2026-07-12 12:18:02.496071	test book part 1	\N
197	Test Book Part 2	eng	2024	novel	Second test book created via API	0	2026-07-12 12:18:02.503149	2026-07-12 12:18:02.503149	test book part 2	\N
872	Updated Book Title	eng	2022	novel	Updated description	0	2026-07-21 16:09:00.997458	2026-07-21 16:09:01.005409	updated book title	\N
198	Updated Book Title	eng	2022	novel	Updated description	0	2026-07-12 12:18:02.515928	2026-07-12 12:18:02.522407	updated book title	\N
212	Add Author Test	eng	0	novel		0	2026-07-12 12:18:03.06264	2026-07-18 20:48:03.408316	add author test	\N
203	Book One	eng	0	novel		0	2026-07-12 12:18:02.606438	2026-07-20 20:55:13.308321	book one	\N
882	Remove Authors Test	eng	0	novel		0	2026-07-21 16:09:01.657874	2026-07-21 16:09:01.657874	remove authors test	\N
201	Updated Title Only	eng	0	novel		0	2026-07-12 12:18:02.573093	2026-07-12 12:18:02.579863	updated title only	\N
202	Updated Title Empty ISBN	eng	0	novel		0	2026-07-12 12:18:02.591618	2026-07-12 12:18:02.597429	updated title empty isbn	\N
205	Original Book Title	eng	0	novel		0	2026-07-12 12:18:02.624208	2026-07-12 12:18:02.624208	original book title	\N
206	Updated Title	eng	0	novel		0	2026-07-12 12:18:02.644655	2026-07-12 12:18:02.650944	updated title	\N
207	Corrupted ISBN Test	eng	0	novel		0	2026-07-12 12:18:02.663862	2026-07-12 12:18:02.663862	corrupted isbn test	\N
208	Remove Authors Test	eng	0	novel		0	2026-07-12 12:18:02.997813	2026-07-12 12:18:02.997813	remove authors test	\N
209	Remove Genres Test	eng	0	novel		0	2026-07-12 12:18:03.014685	2026-07-12 12:18:03.014685	remove genres test	\N
210	Remove Tags Test	eng	0	novel		0	2026-07-12 12:18:03.031074	2026-07-12 12:18:03.031074	remove tags test	\N
211	Nil Authors Updated Title	eng	0	novel		0	2026-07-12 12:18:03.047847	2026-07-12 12:18:03.054284	nil authors updated title	\N
213	Test Book Part 1	eng	2023	novel	First test book created via API	0	2026-07-12 12:18:32.256662	2026-07-12 12:18:32.256662	test book part 1	\N
214	Test Book Part 2	eng	2024	novel	Second test book created via API	0	2026-07-12 12:18:32.262755	2026-07-12 12:18:32.262755	test book part 2	\N
204	Book Two	eng	0	novel		0	2026-07-12 12:18:02.612695	2026-07-13 11:39:42.297456	book two	\N
346	Remove Tags Test	eng	0	novel		0	2026-07-15 12:33:21.13349	2026-07-15 12:33:21.13349	remove tags test	\N
215	Updated Book Title	eng	2022	novel	Updated description	0	2026-07-12 12:18:32.272855	2026-07-12 12:18:32.278639	updated book title	\N
883	Remove Genres Test	eng	0	novel		0	2026-07-21 16:09:01.677387	2026-07-21 16:09:01.677387	remove genres test	\N
217	Updated Title	eng	0	novel	New annotation text	0	2026-07-12 12:18:32.308558	2026-07-12 12:18:32.318924	updated title	\N
218	Updated Title Only	eng	0	novel		0	2026-07-12 12:18:32.33166	2026-07-12 12:18:32.337799	updated title only	\N
893	Updated Title Empty ISBN	eng	0	novel		0	2026-07-21 20:55:27.775829	2026-07-21 20:55:27.781835	updated title empty isbn	\N
219	Updated Title Empty ISBN	eng	0	novel		0	2026-07-12 12:18:32.352074	2026-07-12 12:18:32.357852	updated title empty isbn	\N
221	Book Two	eng	0	novel		0	2026-07-12 12:18:32.375111	2026-07-12 12:18:32.375111	book two	\N
222	Original Book Title	eng	0	novel		0	2026-07-12 12:18:32.386993	2026-07-12 12:18:32.386993	original book title	\N
223	Updated Title	eng	0	novel		0	2026-07-12 12:18:32.404918	2026-07-12 12:18:32.412449	updated title	\N
224	Corrupted ISBN Test	eng	0	novel		0	2026-07-12 12:18:32.421379	2026-07-12 12:18:32.421379	corrupted isbn test	\N
225	Remove Authors Test	eng	0	novel		0	2026-07-12 12:18:32.756463	2026-07-12 12:18:32.756463	remove authors test	\N
226	Remove Genres Test	eng	0	novel		0	2026-07-12 12:18:32.775359	2026-07-12 12:18:32.775359	remove genres test	\N
227	Remove Tags Test	eng	0	novel		0	2026-07-12 12:18:32.796392	2026-07-12 12:18:32.796392	remove tags test	\N
229	Add Author Test5	eng	0	novel		0	2026-07-12 12:18:32.826986	2026-07-23 13:17:34.632757	add author test5	\N
228	Nil Authors Updated Title	eng	0	novel		0	2026-07-12 12:18:32.811545	2026-07-12 12:18:32.8184	nil authors updated title	\N
220	Book One	eng	0	novel		0	2026-07-12 12:18:32.369151	2026-07-12 19:31:55.342291	book one	\N
230	Test Book Part 1	eng	2023	novel	First test book created via API	0	2026-07-12 20:10:38.902451	2026-07-12 20:10:38.902451	test book part 1	\N
231	Test Book Part 2	eng	2024	novel	Second test book created via API	0	2026-07-12 20:10:38.909258	2026-07-12 20:10:38.909258	test book part 2	\N
232	Updated Book Title	eng	2022	novel	Updated description	0	2026-07-12 20:10:38.919663	2026-07-12 20:10:38.927411	updated book title	\N
234	Updated Title	eng	0	novel	New annotation text	0	2026-07-12 20:10:38.958668	2026-07-12 20:10:38.968958	updated title	\N
235	Updated Title Only	eng	0	novel		0	2026-07-12 20:10:38.980271	2026-07-12 20:10:38.986725	updated title only	\N
236	Updated Title Empty ISBN	eng	0	novel		0	2026-07-12 20:10:38.99781	2026-07-12 20:10:39.004594	updated title empty isbn	\N
237	Book One	eng	0	novel		0	2026-07-12 20:10:39.015863	2026-07-12 20:10:39.015863	book one	\N
238	Book Two	eng	0	novel		0	2026-07-12 20:10:39.021881	2026-07-12 20:10:39.021881	book two	\N
239	Original Book Title	eng	0	novel		0	2026-07-12 20:10:39.034065	2026-07-12 20:10:39.034065	original book title	\N
240	Updated Title	eng	0	novel		0	2026-07-12 20:10:39.049522	2026-07-12 20:10:39.056442	updated title	\N
241	Corrupted ISBN Test	eng	0	novel		0	2026-07-12 20:10:39.064685	2026-07-12 20:10:39.064685	corrupted isbn test	\N
242	Remove Authors Test	eng	0	novel		0	2026-07-12 20:10:39.366897	2026-07-12 20:10:39.366897	remove authors test	\N
243	Remove Genres Test	eng	0	novel		0	2026-07-12 20:10:39.383509	2026-07-12 20:10:39.383509	remove genres test	\N
244	Remove Tags Test	eng	0	novel		0	2026-07-12 20:10:39.399412	2026-07-12 20:10:39.399412	remove tags test	\N
245	Nil Authors Updated Title	eng	0	novel		0	2026-07-12 20:10:39.4153	2026-07-12 20:10:39.421512	nil authors updated title	\N
246	Add Author Test	eng	0	novel		0	2026-07-12 20:10:39.429535	2026-07-12 20:10:39.429535	add author test	\N
247	Test Book Part 1	eng	2023	novel	First test book created via API	0	2026-07-15 08:44:20.822797	2026-07-15 08:44:20.822797	test book part 1	\N
248	Test Book Part 2	eng	2024	novel	Second test book created via API	0	2026-07-15 08:44:20.829151	2026-07-15 08:44:20.829151	test book part 2	\N
249	Updated Book Title	eng	2022	novel	Updated description	0	2026-07-15 08:44:20.850686	2026-07-15 08:44:20.856851	updated book title	\N
251	Updated Title	eng	0	novel	New annotation text	0	2026-07-15 08:44:20.894197	2026-07-15 08:44:20.904222	updated title	\N
252	Updated Title Only	eng	0	novel		0	2026-07-15 08:44:20.914861	2026-07-15 08:44:20.921807	updated title only	\N
253	Updated Title Empty ISBN	eng	0	novel		0	2026-07-15 08:44:20.932181	2026-07-15 08:44:20.938136	updated title empty isbn	\N
254	Book One	eng	0	novel		0	2026-07-15 08:44:20.947464	2026-07-15 08:44:20.947464	book one	\N
255	Book Two	eng	0	novel		0	2026-07-15 08:44:20.953044	2026-07-15 08:44:20.953044	book two	\N
256	Original Book Title	eng	0	novel		0	2026-07-15 08:44:20.963106	2026-07-15 08:44:20.963106	original book title	\N
257	Updated Title	eng	0	novel		0	2026-07-15 08:44:20.979927	2026-07-15 08:44:20.987498	updated title	\N
258	Corrupted ISBN Test	eng	0	novel		0	2026-07-15 08:44:20.996975	2026-07-15 08:44:20.996975	corrupted isbn test	\N
259	Remove Authors Test	eng	0	novel		0	2026-07-15 08:44:21.341228	2026-07-15 08:44:21.341228	remove authors test	\N
260	Remove Genres Test	eng	0	novel		0	2026-07-15 08:44:21.357192	2026-07-15 08:44:21.357192	remove genres test	\N
261	Remove Tags Test	eng	0	novel		0	2026-07-15 08:44:21.373614	2026-07-15 08:44:21.373614	remove tags test	\N
262	Nil Authors Updated Title	eng	0	novel		0	2026-07-15 08:44:21.390009	2026-07-15 08:44:21.396809	nil authors updated title	\N
263	Add Author Test	eng	0	novel		0	2026-07-15 08:44:21.40457	2026-07-15 08:44:21.40457	add author test	\N
264	Test Book Part 1	eng	2023	novel	First test book created via API	0	2026-07-15 11:10:54.455944	2026-07-15 11:10:54.455944	test book part 1	\N
265	Test Book Part 2	eng	2024	novel	Second test book created via API	0	2026-07-15 11:10:54.462577	2026-07-15 11:10:54.462577	test book part 2	\N
266	Updated Book Title	eng	2022	novel	Updated description	0	2026-07-15 11:10:54.473337	2026-07-15 11:10:54.479369	updated book title	\N
268	Updated Title	eng	0	novel	New annotation text	0	2026-07-15 11:10:54.502927	2026-07-15 11:10:54.51318	updated title	\N
269	Updated Title Only	eng	0	novel		0	2026-07-15 11:10:54.522383	2026-07-15 11:10:54.528566	updated title only	\N
270	Updated Title Empty ISBN	eng	0	novel		0	2026-07-15 11:10:54.53754	2026-07-15 11:10:54.543682	updated title empty isbn	\N
271	Book One	eng	0	novel		0	2026-07-15 11:10:54.552356	2026-07-15 11:10:54.552356	book one	\N
272	Book Two	eng	0	novel		0	2026-07-15 11:10:54.558517	2026-07-15 11:10:54.558517	book two	\N
273	Original Book Title	eng	0	novel		0	2026-07-15 11:10:54.571237	2026-07-15 11:10:54.571237	original book title	\N
274	Updated Title	eng	0	novel		0	2026-07-15 11:10:54.58866	2026-07-15 11:10:54.59527	updated title	\N
275	Corrupted ISBN Test	eng	0	novel		0	2026-07-15 11:10:54.604642	2026-07-15 11:10:54.604642	corrupted isbn test	\N
276	Remove Authors Test	eng	0	novel		0	2026-07-15 11:10:54.892667	2026-07-15 11:10:54.892667	remove authors test	\N
277	Remove Genres Test	eng	0	novel		0	2026-07-15 11:10:54.907261	2026-07-15 11:10:54.907261	remove genres test	\N
278	Remove Tags Test	eng	0	novel		0	2026-07-15 11:10:54.925318	2026-07-15 11:10:54.925318	remove tags test	\N
279	Nil Authors Updated Title	eng	0	novel		0	2026-07-15 11:10:54.939409	2026-07-15 11:10:54.945618	nil authors updated title	\N
280	Add Author Test	eng	0	novel		0	2026-07-15 11:10:54.953817	2026-07-15 11:10:54.953817	add author test	\N
281	Test Book Part 1	eng	2023	novel	First test book created via API	0	2026-07-15 11:17:16.059925	2026-07-15 11:17:16.059925	test book part 1	\N
282	Test Book Part 2	eng	2024	novel	Second test book created via API	0	2026-07-15 11:17:16.06657	2026-07-15 11:17:16.06657	test book part 2	\N
283	Updated Book Title	eng	2022	novel	Updated description	0	2026-07-15 11:17:16.075633	2026-07-15 11:17:16.081617	updated book title	\N
285	Updated Title	eng	0	novel	New annotation text	0	2026-07-15 11:17:16.107962	2026-07-15 11:17:16.118445	updated title	\N
286	Updated Title Only	eng	0	novel		0	2026-07-15 11:17:16.13052	2026-07-15 11:17:16.137005	updated title only	\N
287	Updated Title Empty ISBN	eng	0	novel		0	2026-07-15 11:17:16.14727	2026-07-15 11:17:16.153048	updated title empty isbn	\N
288	Book One	eng	0	novel		0	2026-07-15 11:17:16.161643	2026-07-15 11:17:16.161643	book one	\N
289	Book Two	eng	0	novel		0	2026-07-15 11:17:16.167357	2026-07-15 11:17:16.167357	book two	\N
290	Original Book Title	eng	0	novel		0	2026-07-15 11:17:16.18005	2026-07-15 11:17:16.18005	original book title	\N
291	Updated Title	eng	0	novel		0	2026-07-15 11:17:16.195037	2026-07-15 11:17:16.201318	updated title	\N
292	Corrupted ISBN Test	eng	0	novel		0	2026-07-15 11:17:16.209241	2026-07-15 11:17:16.209241	corrupted isbn test	\N
293	Remove Authors Test	eng	0	novel		0	2026-07-15 11:17:16.483495	2026-07-15 11:17:16.483495	remove authors test	\N
294	Remove Genres Test	eng	0	novel		0	2026-07-15 11:17:16.498616	2026-07-15 11:17:16.498616	remove genres test	\N
295	Remove Tags Test	eng	0	novel		0	2026-07-15 11:17:16.51308	2026-07-15 11:17:16.51308	remove tags test	\N
296	Nil Authors Updated Title	eng	0	novel		0	2026-07-15 11:17:16.526649	2026-07-15 11:17:16.53385	nil authors updated title	\N
297	Add Author Test	eng	0	novel		0	2026-07-15 11:17:16.543109	2026-07-15 11:17:16.543109	add author test	\N
298	Test Book Part 1	eng	2023	novel	First test book created via API	0	2026-07-15 11:25:56.943335	2026-07-15 11:25:56.943335	test book part 1	\N
299	Test Book Part 2	eng	2024	novel	Second test book created via API	0	2026-07-15 11:25:56.949832	2026-07-15 11:25:56.949832	test book part 2	\N
300	Updated Book Title	eng	2022	novel	Updated description	0	2026-07-15 11:25:56.960728	2026-07-15 11:25:56.967544	updated book title	\N
302	Updated Title	eng	0	novel	New annotation text	0	2026-07-15 11:25:56.993289	2026-07-15 11:25:57.003011	updated title	\N
303	Updated Title Only	eng	0	novel		0	2026-07-15 11:25:57.013661	2026-07-15 11:25:57.019829	updated title only	\N
304	Updated Title Empty ISBN	eng	0	novel		0	2026-07-15 11:25:57.03051	2026-07-15 11:25:57.036408	updated title empty isbn	\N
305	Book One	eng	0	novel		0	2026-07-15 11:25:57.046539	2026-07-15 11:25:57.046539	book one	\N
306	Book Two	eng	0	novel		0	2026-07-15 11:25:57.052333	2026-07-15 11:25:57.052333	book two	\N
307	Original Book Title	eng	0	novel		0	2026-07-15 11:25:57.062967	2026-07-15 11:25:57.062967	original book title	\N
308	Updated Title	eng	0	novel		0	2026-07-15 11:25:57.080505	2026-07-15 11:25:57.087199	updated title	\N
309	Corrupted ISBN Test	eng	0	novel		0	2026-07-15 11:25:57.095274	2026-07-15 11:25:57.095274	corrupted isbn test	\N
310	Remove Authors Test	eng	0	novel		0	2026-07-15 11:26:02.404508	2026-07-15 11:26:02.404508	remove authors test	\N
311	Remove Genres Test	eng	0	novel		0	2026-07-15 11:26:02.419639	2026-07-15 11:26:02.419639	remove genres test	\N
312	Remove Tags Test	eng	0	novel		0	2026-07-15 11:26:02.435509	2026-07-15 11:26:02.435509	remove tags test	\N
313	Nil Authors Updated Title	eng	0	novel		0	2026-07-15 11:26:02.451861	2026-07-15 11:26:02.458109	nil authors updated title	\N
315	Test Book Part 1	eng	2023	novel	First test book created via API	0	2026-07-15 12:31:47.060228	2026-07-15 12:31:47.060228	test book part 1	\N
316	Test Book Part 2	eng	2024	novel	Second test book created via API	0	2026-07-15 12:31:47.066953	2026-07-15 12:31:47.066953	test book part 2	\N
317	Updated Book Title	eng	2022	novel	Updated description	0	2026-07-15 12:31:47.077078	2026-07-15 12:31:47.083869	updated book title	\N
319	Updated Title	eng	0	novel	New annotation text	0	2026-07-15 12:31:47.110753	2026-07-15 12:31:47.120392	updated title	\N
320	Updated Title Only	eng	0	novel		0	2026-07-15 12:31:47.135188	2026-07-15 12:31:47.140936	updated title only	\N
321	Updated Title Empty ISBN	eng	0	novel		0	2026-07-15 12:31:47.150268	2026-07-15 12:31:47.156164	updated title empty isbn	\N
322	Book One	eng	0	novel		0	2026-07-15 12:31:47.17389	2026-07-15 12:31:47.17389	book one	\N
323	Book Two	eng	0	novel		0	2026-07-15 12:31:47.179814	2026-07-15 12:31:47.179814	book two	\N
324	Original Book Title	eng	0	novel		0	2026-07-15 12:31:47.191611	2026-07-15 12:31:47.191611	original book title	\N
325	Updated Title	eng	0	novel		0	2026-07-15 12:31:47.207282	2026-07-15 12:31:47.214094	updated title	\N
326	Corrupted ISBN Test	eng	0	novel		0	2026-07-15 12:31:47.222	2026-07-15 12:31:47.222	corrupted isbn test	\N
327	Remove Authors Test	eng	0	novel		0	2026-07-15 12:31:47.523246	2026-07-15 12:31:47.523246	remove authors test	\N
328	Remove Genres Test	eng	0	novel		0	2026-07-15 12:31:47.541337	2026-07-15 12:31:47.541337	remove genres test	\N
329	Remove Tags Test	eng	0	novel		0	2026-07-15 12:31:47.558714	2026-07-15 12:31:47.558714	remove tags test	\N
330	Nil Authors Updated Title	eng	0	novel		0	2026-07-15 12:31:47.575156	2026-07-15 12:31:47.582141	nil authors updated title	\N
331	Add Author Test	eng	0	novel		0	2026-07-15 12:31:47.59113	2026-07-15 12:31:47.59113	add author test	\N
332	Test Book Part 1	eng	2023	novel	First test book created via API	0	2026-07-15 12:33:20.39546	2026-07-15 12:33:20.39546	test book part 1	\N
333	Test Book Part 2	eng	2024	novel	Second test book created via API	0	2026-07-15 12:33:20.40135	2026-07-15 12:33:20.40135	test book part 2	\N
334	Updated Book Title	eng	2022	novel	Updated description	0	2026-07-15 12:33:20.412097	2026-07-15 12:33:20.417823	updated book title	\N
336	Updated Title	eng	0	novel	New annotation text	0	2026-07-15 12:33:20.444583	2026-07-15 12:33:20.455004	updated title	\N
337	Updated Title Only	eng	0	novel		0	2026-07-15 12:33:20.666521	2026-07-15 12:33:20.672562	updated title only	\N
338	Updated Title Empty ISBN	eng	0	novel		0	2026-07-15 12:33:20.683895	2026-07-15 12:33:20.690001	updated title empty isbn	\N
339	Book One	eng	0	novel		0	2026-07-15 12:33:20.701301	2026-07-15 12:33:20.701301	book one	\N
340	Book Two	eng	0	novel		0	2026-07-15 12:33:20.707766	2026-07-15 12:33:20.707766	book two	\N
341	Original Book Title	eng	0	novel		0	2026-07-15 12:33:20.720266	2026-07-15 12:33:20.720266	original book title	\N
342	Updated Title	eng	0	novel		0	2026-07-15 12:33:20.737465	2026-07-15 12:33:20.743687	updated title	\N
343	Corrupted ISBN Test	eng	0	novel		0	2026-07-15 12:33:20.752633	2026-07-15 12:33:20.752633	corrupted isbn test	\N
344	Remove Authors Test	eng	0	novel		0	2026-07-15 12:33:21.100618	2026-07-15 12:33:21.100618	remove authors test	\N
345	Remove Genres Test	eng	0	novel		0	2026-07-15 12:33:21.11749	2026-07-15 12:33:21.11749	remove genres test	\N
347	Nil Authors Updated Title	eng	0	novel		0	2026-07-15 12:33:21.147393	2026-07-15 12:33:21.153054	nil authors updated title	\N
349	Test Book Part 1	eng	2023	novel	First test book created via API	0	2026-07-15 12:38:23.826639	2026-07-15 12:38:23.826639	test book part 1	\N
350	Test Book Part 2	eng	2024	novel	Second test book created via API	0	2026-07-15 12:38:23.833383	2026-07-15 12:38:23.833383	test book part 2	\N
351	Updated Book Title	eng	2022	novel	Updated description	0	2026-07-15 12:38:23.845596	2026-07-15 12:38:23.851519	updated book title	\N
353	Updated Title	eng	0	novel	New annotation text	0	2026-07-15 12:38:23.881484	2026-07-15 12:38:23.891117	updated title	\N
354	Updated Title Only	eng	0	novel		0	2026-07-15 12:38:23.905436	2026-07-15 12:38:23.911309	updated title only	\N
355	Updated Title Empty ISBN	eng	0	novel		0	2026-07-15 12:38:23.921829	2026-07-15 12:38:23.9274	updated title empty isbn	\N
356	Book One	eng	0	novel		0	2026-07-15 12:38:23.942764	2026-07-15 12:38:23.942764	book one	\N
357	Book Two	eng	0	novel		0	2026-07-15 12:38:23.949812	2026-07-15 12:38:23.949812	book two	\N
358	Original Book Title	eng	0	novel		0	2026-07-15 12:38:23.961966	2026-07-15 12:38:23.961966	original book title	\N
359	Updated Title	eng	0	novel		0	2026-07-15 12:38:23.982726	2026-07-15 12:38:23.988784	updated title	\N
360	Corrupted ISBN Test	eng	0	novel		0	2026-07-15 12:38:23.9982	2026-07-15 12:38:23.9982	corrupted isbn test	\N
361	Remove Authors Test	eng	0	novel		0	2026-07-15 12:38:24.325114	2026-07-15 12:38:24.325114	remove authors test	\N
362	Remove Genres Test	eng	0	novel		0	2026-07-15 12:38:24.341511	2026-07-15 12:38:24.341511	remove genres test	\N
363	Remove Tags Test	eng	0	novel		0	2026-07-15 12:38:24.378098	2026-07-15 12:38:24.378098	remove tags test	\N
364	Nil Authors Updated Title	eng	0	novel		0	2026-07-15 12:38:24.395247	2026-07-15 12:38:24.401478	nil authors updated title	\N
365	Add Author Test	eng	0	novel		0	2026-07-15 12:38:24.409845	2026-07-15 12:38:24.409845	add author test	\N
366	Test Book Part 1	eng	2023	novel	First test book created via API	0	2026-07-15 13:22:41.042904	2026-07-15 13:22:41.042904	test book part 1	\N
367	Test Book Part 2	eng	2024	novel	Second test book created via API	0	2026-07-15 13:22:41.048772	2026-07-15 13:22:41.048772	test book part 2	\N
368	Updated Book Title	eng	2022	novel	Updated description	0	2026-07-15 13:22:41.05998	2026-07-15 13:22:41.066432	updated book title	\N
370	Updated Title	eng	0	novel	New annotation text	0	2026-07-15 13:22:41.094024	2026-07-15 13:22:41.10398	updated title	\N
371	Updated Title Only	eng	0	novel		0	2026-07-15 13:22:41.115042	2026-07-15 13:22:41.120636	updated title only	\N
372	Updated Title Empty ISBN	eng	0	novel		0	2026-07-15 13:22:41.131793	2026-07-15 13:22:41.137574	updated title empty isbn	\N
373	Book One	eng	0	novel		0	2026-07-15 13:22:41.147237	2026-07-15 13:22:41.147237	book one	\N
374	Book Two	eng	0	novel		0	2026-07-15 13:22:41.153154	2026-07-15 13:22:41.153154	book two	\N
375	Original Book Title	eng	0	novel		0	2026-07-15 13:22:41.164743	2026-07-15 13:22:41.164743	original book title	\N
376	Updated Title	eng	0	novel		0	2026-07-15 13:22:41.181428	2026-07-15 13:22:41.187568	updated title	\N
377	Corrupted ISBN Test	eng	0	novel		0	2026-07-15 13:22:41.202014	2026-07-15 13:22:41.202014	corrupted isbn test	\N
378	Remove Authors Test	eng	0	novel		0	2026-07-15 13:22:41.525658	2026-07-15 13:22:41.525658	remove authors test	\N
379	Remove Genres Test	eng	0	novel		0	2026-07-15 13:22:41.542953	2026-07-15 13:22:41.542953	remove genres test	\N
380	Remove Tags Test	eng	0	novel		0	2026-07-15 13:22:41.559167	2026-07-15 13:22:41.559167	remove tags test	\N
381	Nil Authors Updated Title	eng	0	novel		0	2026-07-15 13:22:41.574774	2026-07-15 13:22:41.580557	nil authors updated title	\N
383	Test Book Part 1	eng	2023	novel	First test book created via API	0	2026-07-15 14:50:32.622829	2026-07-15 14:50:32.622829	test book part 1	\N
384	Test Book Part 2	eng	2024	novel	Second test book created via API	0	2026-07-15 14:50:32.629095	2026-07-15 14:50:32.629095	test book part 2	\N
385	Updated Book Title	eng	2022	novel	Updated description	0	2026-07-15 14:50:32.639384	2026-07-15 14:50:32.645193	updated book title	\N
387	Updated Title	eng	0	novel	New annotation text	0	2026-07-15 14:50:32.675058	2026-07-15 14:50:32.686137	updated title	\N
388	Updated Title Only	eng	0	novel		0	2026-07-15 14:50:32.697854	2026-07-15 14:50:32.703378	updated title only	\N
389	Updated Title Empty ISBN	eng	0	novel		0	2026-07-15 14:50:32.716891	2026-07-15 14:50:32.722475	updated title empty isbn	\N
390	Book One	eng	0	novel		0	2026-07-15 14:50:32.733255	2026-07-15 14:50:32.733255	book one	\N
391	Book Two	eng	0	novel		0	2026-07-15 14:50:32.738559	2026-07-15 14:50:32.738559	book two	\N
392	Original Book Title	eng	0	novel		0	2026-07-15 14:50:32.751238	2026-07-15 14:50:32.751238	original book title	\N
393	Updated Title	eng	0	novel		0	2026-07-15 14:50:32.766676	2026-07-15 14:50:32.772657	updated title	\N
394	Corrupted ISBN Test	eng	0	novel		0	2026-07-15 14:50:32.782569	2026-07-15 14:50:32.782569	corrupted isbn test	\N
395	Remove Authors Test	eng	0	novel		0	2026-07-15 14:50:33.095301	2026-07-15 14:50:33.095301	remove authors test	\N
396	Remove Genres Test	eng	0	novel		0	2026-07-15 14:50:33.11146	2026-07-15 14:50:33.11146	remove genres test	\N
397	Remove Tags Test	eng	0	novel		0	2026-07-15 14:50:33.12665	2026-07-15 14:50:33.12665	remove tags test	\N
398	Nil Authors Updated Title	eng	0	novel		0	2026-07-15 14:50:33.145271	2026-07-15 14:50:33.15103	nil authors updated title	\N
399	Add Author Test	eng	0	novel		0	2026-07-15 14:50:33.159857	2026-07-15 14:50:33.159857	add author test	\N
400	Test Book Part 1	eng	2023	novel	First test book created via API	0	2026-07-15 14:57:25.212397	2026-07-15 14:57:25.212397	test book part 1	\N
401	Test Book Part 2	eng	2024	novel	Second test book created via API	0	2026-07-15 14:57:25.218584	2026-07-15 14:57:25.218584	test book part 2	\N
402	Updated Book Title	eng	2022	novel	Updated description	0	2026-07-15 14:57:25.230155	2026-07-15 14:57:25.235821	updated book title	\N
404	Updated Title	eng	0	novel	New annotation text	0	2026-07-15 14:57:25.263708	2026-07-15 14:57:25.274417	updated title	\N
405	Updated Title Only	eng	0	novel		0	2026-07-15 14:57:25.287246	2026-07-15 14:57:25.292911	updated title only	\N
406	Updated Title Empty ISBN	eng	0	novel		0	2026-07-15 14:57:25.305507	2026-07-15 14:57:25.311884	updated title empty isbn	\N
407	Book One	eng	0	novel		0	2026-07-15 14:57:25.321305	2026-07-15 14:57:25.321305	book one	\N
408	Book Two	eng	0	novel		0	2026-07-15 14:57:25.32718	2026-07-15 14:57:25.32718	book two	\N
409	Original Book Title	eng	0	novel		0	2026-07-15 14:57:25.340215	2026-07-15 14:57:25.340215	original book title	\N
410	Updated Title	eng	0	novel		0	2026-07-15 14:57:25.356639	2026-07-15 14:57:25.363071	updated title	\N
411	Corrupted ISBN Test	eng	0	novel		0	2026-07-15 14:57:25.372583	2026-07-15 14:57:25.372583	corrupted isbn test	\N
412	Remove Authors Test	eng	0	novel		0	2026-07-15 14:57:25.692501	2026-07-15 14:57:25.692501	remove authors test	\N
413	Remove Genres Test	eng	0	novel		0	2026-07-15 14:57:25.709233	2026-07-15 14:57:25.709233	remove genres test	\N
414	Remove Tags Test	eng	0	novel		0	2026-07-15 14:57:25.725789	2026-07-15 14:57:25.725789	remove tags test	\N
415	Nil Authors Updated Title	eng	0	novel		0	2026-07-15 14:57:25.741801	2026-07-15 14:57:25.747786	nil authors updated title	\N
417	Test Book Part 1	eng	2023	novel	First test book created via API	0	2026-07-15 15:01:46.273532	2026-07-15 15:01:46.273532	test book part 1	\N
418	Test Book Part 2	eng	2024	novel	Second test book created via API	0	2026-07-15 15:01:46.279582	2026-07-15 15:01:46.279582	test book part 2	\N
419	Updated Book Title	eng	2022	novel	Updated description	0	2026-07-15 15:01:46.290162	2026-07-15 15:01:46.296215	updated book title	\N
421	Updated Title	eng	0	novel	New annotation text	0	2026-07-15 15:01:46.326025	2026-07-15 15:01:46.336483	updated title	\N
422	Updated Title Only	eng	0	novel		0	2026-07-15 15:01:46.348715	2026-07-15 15:01:46.355041	updated title only	\N
423	Updated Title Empty ISBN	eng	0	novel		0	2026-07-15 15:01:46.366337	2026-07-15 15:01:46.372406	updated title empty isbn	\N
424	Book One	eng	0	novel		0	2026-07-15 15:01:46.38426	2026-07-15 15:01:46.38426	book one	\N
425	Book Two	eng	0	novel		0	2026-07-15 15:01:46.389854	2026-07-15 15:01:46.389854	book two	\N
426	Original Book Title	eng	0	novel		0	2026-07-15 15:01:46.412626	2026-07-15 15:01:46.412626	original book title	\N
427	Updated Title	eng	0	novel		0	2026-07-15 15:01:46.432084	2026-07-15 15:01:46.438908	updated title	\N
428	Corrupted ISBN Test	eng	0	novel		0	2026-07-15 15:01:46.450281	2026-07-15 15:01:46.450281	corrupted isbn test	\N
429	Remove Authors Test	eng	0	novel		0	2026-07-15 15:01:46.882943	2026-07-15 15:01:46.882943	remove authors test	\N
430	Remove Genres Test	eng	0	novel		0	2026-07-15 15:01:46.899738	2026-07-15 15:01:46.899738	remove genres test	\N
431	Remove Tags Test	eng	0	novel		0	2026-07-15 15:01:46.9163	2026-07-15 15:01:46.9163	remove tags test	\N
432	Nil Authors Updated Title	eng	0	novel		0	2026-07-15 15:01:46.931549	2026-07-15 15:01:46.937583	nil authors updated title	\N
433	Add Author Test	eng	0	novel		0	2026-07-15 15:01:46.946302	2026-07-15 15:01:46.946302	add author test	\N
434	Test Book Part 1	eng	2023	novel	First test book created via API	0	2026-07-15 15:44:42.120611	2026-07-15 15:44:42.120611	test book part 1	\N
435	Test Book Part 2	eng	2024	novel	Second test book created via API	0	2026-07-15 15:44:42.126907	2026-07-15 15:44:42.126907	test book part 2	\N
436	Updated Book Title	eng	2022	novel	Updated description	0	2026-07-15 15:44:42.137584	2026-07-15 15:44:42.143027	updated book title	\N
438	Updated Title	eng	0	novel	New annotation text	0	2026-07-15 15:44:42.170426	2026-07-15 15:44:42.181466	updated title	\N
439	Updated Title Only	eng	0	novel		0	2026-07-15 15:44:42.194225	2026-07-15 15:44:42.200007	updated title only	\N
440	Updated Title Empty ISBN	eng	0	novel		0	2026-07-15 15:44:42.210132	2026-07-15 15:44:42.215562	updated title empty isbn	\N
441	Book One	eng	0	novel		0	2026-07-15 15:44:42.224617	2026-07-15 15:44:42.224617	book one	\N
442	Book Two	eng	0	novel		0	2026-07-15 15:44:42.230332	2026-07-15 15:44:42.230332	book two	\N
443	Original Book Title	eng	0	novel		0	2026-07-15 15:44:42.241793	2026-07-15 15:44:42.241793	original book title	\N
444	Updated Title	eng	0	novel		0	2026-07-15 15:44:42.259819	2026-07-15 15:44:42.266022	updated title	\N
445	Corrupted ISBN Test	eng	0	novel		0	2026-07-15 15:44:42.275485	2026-07-15 15:44:42.275485	corrupted isbn test	\N
446	Remove Authors Test	eng	0	novel		0	2026-07-15 15:44:42.699648	2026-07-15 15:44:42.699648	remove authors test	\N
447	Remove Genres Test	eng	0	novel		0	2026-07-15 15:44:42.715546	2026-07-15 15:44:42.715546	remove genres test	\N
448	Remove Tags Test	eng	0	novel		0	2026-07-15 15:44:42.733712	2026-07-15 15:44:42.733712	remove tags test	\N
449	Nil Authors Updated Title	eng	0	novel		0	2026-07-15 15:44:42.749068	2026-07-15 15:44:42.754901	nil authors updated title	\N
450	Add Author Test	eng	0	novel		0	2026-07-15 15:44:42.763748	2026-07-15 15:44:42.763748	add author test	\N
451	Test Book Part 1	eng	2023	novel	First test book created via API	0	2026-07-16 00:00:18.325954	2026-07-16 00:00:18.325954	test book part 1	\N
452	Test Book Part 2	eng	2024	novel	Second test book created via API	0	2026-07-16 00:00:18.331879	2026-07-16 00:00:18.331879	test book part 2	\N
453	Updated Book Title	eng	2022	novel	Updated description	0	2026-07-16 00:00:18.342737	2026-07-16 00:00:18.348497	updated book title	\N
455	Updated Title	eng	0	novel	New annotation text	0	2026-07-16 00:00:18.375011	2026-07-16 00:00:18.384964	updated title	\N
456	Updated Title Only	eng	0	novel		0	2026-07-16 00:00:18.397376	2026-07-16 00:00:18.404303	updated title only	\N
457	Updated Title Empty ISBN	eng	0	novel		0	2026-07-16 00:00:18.415814	2026-07-16 00:00:18.421475	updated title empty isbn	\N
458	Book One	eng	0	novel		0	2026-07-16 00:00:18.430578	2026-07-16 00:00:18.430578	book one	\N
459	Book Two	eng	0	novel		0	2026-07-16 00:00:18.436432	2026-07-16 00:00:18.436432	book two	\N
460	Original Book Title	eng	0	novel		0	2026-07-16 00:00:18.452206	2026-07-16 00:00:18.452206	original book title	\N
461	Updated Title	eng	0	novel		0	2026-07-16 00:00:18.469925	2026-07-16 00:00:18.476001	updated title	\N
462	Corrupted ISBN Test	eng	0	novel		0	2026-07-16 00:00:18.486004	2026-07-16 00:00:18.486004	corrupted isbn test	\N
463	Remove Authors Test	eng	0	novel		0	2026-07-16 00:00:18.91478	2026-07-16 00:00:18.91478	remove authors test	\N
464	Remove Genres Test	eng	0	novel		0	2026-07-16 00:00:18.931826	2026-07-16 00:00:18.931826	remove genres test	\N
465	Remove Tags Test	eng	0	novel		0	2026-07-16 00:00:18.947752	2026-07-16 00:00:18.947752	remove tags test	\N
466	Nil Authors Updated Title	eng	0	novel		0	2026-07-16 00:00:18.962742	2026-07-16 00:00:18.969831	nil authors updated title	\N
467	Add Author Test	eng	0	novel		0	2026-07-16 00:00:18.979901	2026-07-16 00:00:18.979901	add author test	\N
468	Test Book Part 1	eng	2023	novel	First test book created via API	0	2026-07-16 05:13:10.193857	2026-07-16 05:13:10.193857	test book part 1	\N
469	Test Book Part 2	eng	2024	novel	Second test book created via API	0	2026-07-16 05:13:10.199859	2026-07-16 05:13:10.199859	test book part 2	\N
470	Updated Book Title	eng	2022	novel	Updated description	0	2026-07-16 05:13:10.215997	2026-07-16 05:13:10.221953	updated book title	\N
472	Updated Title	eng	0	novel	New annotation text	0	2026-07-16 05:13:10.27087	2026-07-16 05:13:10.2816	updated title	\N
473	Updated Title Only	eng	0	novel		0	2026-07-16 05:13:10.297237	2026-07-16 05:13:10.302923	updated title only	\N
474	Updated Title Empty ISBN	eng	0	novel		0	2026-07-16 05:13:10.316674	2026-07-16 05:13:10.322301	updated title empty isbn	\N
475	Book One	eng	0	novel		0	2026-07-16 05:13:10.340774	2026-07-16 05:13:10.340774	book one	\N
476	Book Two	eng	0	novel		0	2026-07-16 05:13:10.346708	2026-07-16 05:13:10.346708	book two	\N
477	Original Book Title	eng	0	novel		0	2026-07-16 05:13:10.365052	2026-07-16 05:13:10.365052	original book title	\N
478	Updated Title	eng	0	novel		0	2026-07-16 05:13:10.385207	2026-07-16 05:13:10.391705	updated title	\N
479	Corrupted ISBN Test	eng	0	novel		0	2026-07-16 05:13:10.403912	2026-07-16 05:13:10.403912	corrupted isbn test	\N
480	Remove Authors Test	eng	0	novel		0	2026-07-16 05:13:11.005959	2026-07-16 05:13:11.005959	remove authors test	\N
481	Remove Genres Test	eng	0	novel		0	2026-07-16 05:13:11.024593	2026-07-16 05:13:11.024593	remove genres test	\N
482	Remove Tags Test	eng	0	novel		0	2026-07-16 05:13:11.04492	2026-07-16 05:13:11.04492	remove tags test	\N
535	Add Author Test	eng	0	novel		0	2026-07-16 13:48:26.276186	2026-07-18 07:31:35.204262	add author test	\N
483	Nil Authors Updated Title	eng	0	novel		0	2026-07-16 05:13:11.063093	2026-07-16 05:13:11.069895	nil authors updated title	\N
484	Add Author Test	eng	0	novel		0	2026-07-16 05:13:11.081085	2026-07-16 05:13:11.081085	add author test	\N
485	Test Book Part 1	eng	2023	novel	First test book created via API	0	2026-07-16 05:19:07.84064	2026-07-16 05:19:07.84064	test book part 1	\N
486	Test Book Part 2	eng	2024	novel	Second test book created via API	0	2026-07-16 05:19:07.847173	2026-07-16 05:19:07.847173	test book part 2	\N
487	Updated Book Title	eng	2022	novel	Updated description	0	2026-07-16 05:19:07.861986	2026-07-16 05:19:07.86806	updated book title	\N
489	Updated Title	eng	0	novel	New annotation text	0	2026-07-16 05:19:07.901903	2026-07-16 05:19:07.912173	updated title	\N
490	Updated Title Only	eng	0	novel		0	2026-07-16 05:19:07.926913	2026-07-16 05:19:07.932617	updated title only	\N
491	Updated Title Empty ISBN	eng	0	novel		0	2026-07-16 05:19:07.948587	2026-07-16 05:19:07.954782	updated title empty isbn	\N
492	Book One	eng	0	novel		0	2026-07-16 05:19:07.967495	2026-07-16 05:19:07.967495	book one	\N
493	Book Two	eng	0	novel		0	2026-07-16 05:19:07.973958	2026-07-16 05:19:07.973958	book two	\N
494	Original Book Title	eng	0	novel		0	2026-07-16 05:19:07.991001	2026-07-16 05:19:07.991001	original book title	\N
495	Updated Title	eng	0	novel		0	2026-07-16 05:19:08.012593	2026-07-16 05:19:08.018921	updated title	\N
496	Corrupted ISBN Test	eng	0	novel		0	2026-07-16 05:19:08.031522	2026-07-16 05:19:08.031522	corrupted isbn test	\N
497	Remove Authors Test	eng	0	novel		0	2026-07-16 05:19:08.649389	2026-07-16 05:19:08.649389	remove authors test	\N
498	Remove Genres Test	eng	0	novel		0	2026-07-16 05:19:08.668586	2026-07-16 05:19:08.668586	remove genres test	\N
499	Remove Tags Test	eng	0	novel		0	2026-07-16 05:19:08.689174	2026-07-16 05:19:08.689174	remove tags test	\N
500	Nil Authors Updated Title	eng	0	novel		0	2026-07-16 05:19:08.709535	2026-07-16 05:19:08.716749	nil authors updated title	\N
501	Add Author Test	eng	0	novel		0	2026-07-16 05:19:08.728588	2026-07-16 05:19:08.728588	add author test	\N
502	Test Book Part 1	eng	2023	novel	First test book created via API	0	2026-07-16 09:51:24.257907	2026-07-16 09:51:24.257907	test book part 1	\N
503	Test Book Part 2	eng	2024	novel	Second test book created via API	0	2026-07-16 09:51:24.263958	2026-07-16 09:51:24.263958	test book part 2	\N
504	Updated Book Title	eng	2022	novel	Updated description	0	2026-07-16 09:51:24.276236	2026-07-16 09:51:24.28262	updated book title	\N
506	Updated Title	eng	0	novel	New annotation text	0	2026-07-16 09:51:24.312074	2026-07-16 09:51:24.321458	updated title	\N
507	Updated Title Only	eng	0	novel		0	2026-07-16 09:51:24.334036	2026-07-16 09:51:24.340644	updated title only	\N
508	Updated Title Empty ISBN	eng	0	novel		0	2026-07-16 09:51:24.351483	2026-07-16 09:51:24.357247	updated title empty isbn	\N
509	Book One	eng	0	novel		0	2026-07-16 09:51:24.367311	2026-07-16 09:51:24.367311	book one	\N
510	Book Two	eng	0	novel		0	2026-07-16 09:51:24.373909	2026-07-16 09:51:24.373909	book two	\N
511	Original Book Title	eng	0	novel		0	2026-07-16 09:51:24.3866	2026-07-16 09:51:24.3866	original book title	\N
512	Updated Title	eng	0	novel		0	2026-07-16 09:51:24.403205	2026-07-16 09:51:24.409875	updated title	\N
513	Corrupted ISBN Test	eng	0	novel		0	2026-07-16 09:51:24.41911	2026-07-16 09:51:24.41911	corrupted isbn test	\N
514	Remove Authors Test	eng	0	novel		0	2026-07-16 09:51:24.836834	2026-07-16 09:51:24.836834	remove authors test	\N
515	Remove Genres Test	eng	0	novel		0	2026-07-16 09:51:24.853991	2026-07-16 09:51:24.853991	remove genres test	\N
516	Remove Tags Test	eng	0	novel		0	2026-07-16 09:51:24.870654	2026-07-16 09:51:24.870654	remove tags test	\N
517	Nil Authors Updated Title	eng	0	novel		0	2026-07-16 09:51:24.885862	2026-07-16 09:51:24.891728	nil authors updated title	\N
518	Add Author Test	eng	0	novel		0	2026-07-16 09:51:24.900963	2026-07-16 09:51:24.900963	add author test	\N
519	Test Book Part 1	eng	2023	novel	First test book created via API	0	2026-07-16 13:48:25.641523	2026-07-16 13:48:25.641523	test book part 1	\N
520	Test Book Part 2	eng	2024	novel	Second test book created via API	0	2026-07-16 13:48:25.647507	2026-07-16 13:48:25.647507	test book part 2	\N
521	Updated Book Title	eng	2022	novel	Updated description	0	2026-07-16 13:48:25.659262	2026-07-16 13:48:25.664817	updated book title	\N
523	Updated Title	eng	0	novel	New annotation text	0	2026-07-16 13:48:25.691298	2026-07-16 13:48:25.701588	updated title	\N
524	Updated Title Only	eng	0	novel		0	2026-07-16 13:48:25.713685	2026-07-16 13:48:25.719623	updated title only	\N
525	Updated Title Empty ISBN	eng	0	novel		0	2026-07-16 13:48:25.732219	2026-07-16 13:48:25.737682	updated title empty isbn	\N
526	Book One	eng	0	novel		0	2026-07-16 13:48:25.748534	2026-07-16 13:48:25.748534	book one	\N
527	Book Two	eng	0	novel		0	2026-07-16 13:48:25.754315	2026-07-16 13:48:25.754315	book two	\N
528	Original Book Title	eng	0	novel		0	2026-07-16 13:48:25.765161	2026-07-16 13:48:25.765161	original book title	\N
529	Updated Title	eng	0	novel		0	2026-07-16 13:48:25.781191	2026-07-16 13:48:25.787181	updated title	\N
530	Corrupted ISBN Test	eng	0	novel		0	2026-07-16 13:48:25.796213	2026-07-16 13:48:25.796213	corrupted isbn test	\N
531	Remove Authors Test	eng	0	novel		0	2026-07-16 13:48:26.214669	2026-07-16 13:48:26.214669	remove authors test	\N
532	Remove Genres Test	eng	0	novel		0	2026-07-16 13:48:26.230894	2026-07-16 13:48:26.230894	remove genres test	\N
533	Remove Tags Test	eng	0	novel		0	2026-07-16 13:48:26.246806	2026-07-16 13:48:26.246806	remove tags test	\N
534	Nil Authors Updated Title	eng	0	novel		0	2026-07-16 13:48:26.261889	2026-07-16 13:48:26.267707	nil authors updated title	\N
536	Test Book Part 1	eng	2023	novel	First test book created via API	0	2026-07-16 14:37:13.639082	2026-07-16 14:37:13.639082	test book part 1	\N
537	Test Book Part 2	eng	2024	novel	Second test book created via API	0	2026-07-16 14:37:13.645387	2026-07-16 14:37:13.645387	test book part 2	\N
538	Updated Book Title	eng	2022	novel	Updated description	0	2026-07-16 14:37:13.656564	2026-07-16 14:37:13.662324	updated book title	\N
540	Updated Title	eng	0	novel	New annotation text	0	2026-07-16 14:37:13.688819	2026-07-16 14:37:13.700505	updated title	\N
541	Updated Title Only	eng	0	novel		0	2026-07-16 14:37:13.722114	2026-07-16 14:37:13.728361	updated title only	\N
542	Updated Title Empty ISBN	eng	0	novel		0	2026-07-16 14:37:13.741516	2026-07-16 14:37:13.746909	updated title empty isbn	\N
543	Book One	eng	0	novel		0	2026-07-16 14:37:13.757441	2026-07-16 14:37:13.757441	book one	\N
544	Book Two	eng	0	novel		0	2026-07-16 14:37:13.763147	2026-07-16 14:37:13.763147	book two	\N
545	Original Book Title	eng	0	novel		0	2026-07-16 14:37:13.774287	2026-07-16 14:37:13.774287	original book title	\N
546	Updated Title	eng	0	novel		0	2026-07-16 14:37:13.790365	2026-07-16 14:37:13.796409	updated title	\N
547	Corrupted ISBN Test	eng	0	novel		0	2026-07-16 14:37:13.816803	2026-07-16 14:37:13.816803	corrupted isbn test	\N
548	Remove Authors Test	eng	0	novel		0	2026-07-16 14:37:14.240742	2026-07-16 14:37:14.240742	remove authors test	\N
549	Remove Genres Test	eng	0	novel		0	2026-07-16 14:37:14.257462	2026-07-16 14:37:14.257462	remove genres test	\N
550	Remove Tags Test	eng	0	novel		0	2026-07-16 14:37:14.272864	2026-07-16 14:37:14.272864	remove tags test	\N
555	Updated Book Title	eng	2022	novel	Updated description	0	2026-07-16 14:42:15.245018	2026-07-16 14:42:15.25122	updated book title	\N
551	Nil Authors Updated Title	eng	0	novel		0	2026-07-16 14:37:14.290553	2026-07-16 14:37:14.296478	nil authors updated title	\N
557	Updated Title	eng	0	novel	New annotation text	0	2026-07-16 14:42:15.279652	2026-07-16 14:42:15.291714	updated title	\N
558	Updated Title Only	eng	0	novel		0	2026-07-16 14:42:15.305864	2026-07-16 14:42:15.312186	updated title only	\N
559	Updated Title Empty ISBN	eng	0	novel		0	2026-07-16 14:42:15.324802	2026-07-16 14:42:15.330523	updated title empty isbn	\N
560	Book One	eng	0	novel		0	2026-07-16 14:42:15.339339	2026-07-16 14:42:15.339339	book one	\N
561	Book Two	eng	0	novel		0	2026-07-16 14:42:15.345205	2026-07-16 14:42:15.345205	book two	\N
562	Original Book Title	eng	0	novel		0	2026-07-16 14:42:15.35659	2026-07-16 14:42:15.35659	original book title	\N
563	Updated Title	eng	0	novel		0	2026-07-16 14:42:15.372357	2026-07-16 14:42:15.379062	updated title	\N
564	Corrupted ISBN Test	eng	0	novel		0	2026-07-16 14:42:15.388905	2026-07-16 14:42:15.388905	corrupted isbn test	\N
565	Remove Authors Test	eng	0	novel		0	2026-07-16 14:42:15.845192	2026-07-16 14:42:15.845192	remove authors test	\N
566	Remove Genres Test	eng	0	novel		0	2026-07-16 14:42:15.864239	2026-07-16 14:42:15.864239	remove genres test	\N
567	Remove Tags Test	eng	0	novel		0	2026-07-16 14:42:15.87988	2026-07-16 14:42:15.87988	remove tags test	\N
568	Nil Authors Updated Title	eng	0	novel		0	2026-07-16 14:42:15.895821	2026-07-16 14:42:15.901813	nil authors updated title	\N
569	Add Author Test	eng	0	novel		0	2026-07-16 14:42:15.911775	2026-07-16 14:42:15.911775	add author test	\N
570	Test Book Part 1	eng	2023	novel	First test book created via API	0	2026-07-16 15:13:29.931365	2026-07-16 15:13:29.931365	test book part 1	\N
571	Test Book Part 2	eng	2024	novel	Second test book created via API	0	2026-07-16 15:13:29.939036	2026-07-16 15:13:29.939036	test book part 2	\N
572	Updated Book Title	eng	2022	novel	Updated description	0	2026-07-16 15:13:29.949942	2026-07-16 15:13:29.957681	updated book title	\N
574	Updated Title	eng	0	novel	New annotation text	0	2026-07-16 15:13:29.986276	2026-07-16 15:13:29.996077	updated title	\N
575	Updated Title Only	eng	0	novel		0	2026-07-16 15:13:30.008294	2026-07-16 15:13:30.014126	updated title only	\N
576	Updated Title Empty ISBN	eng	0	novel		0	2026-07-16 15:13:30.024446	2026-07-16 15:13:30.031105	updated title empty isbn	\N
577	Book One	eng	0	novel		0	2026-07-16 15:13:30.040315	2026-07-16 15:13:30.040315	book one	\N
578	Book Two	eng	0	novel		0	2026-07-16 15:13:30.046638	2026-07-16 15:13:30.046638	book two	\N
579	Original Book Title	eng	0	novel		0	2026-07-16 15:13:30.058745	2026-07-16 15:13:30.058745	original book title	\N
580	Updated Title	eng	0	novel		0	2026-07-16 15:13:30.078098	2026-07-16 15:13:30.084675	updated title	\N
581	Corrupted ISBN Test	eng	0	novel		0	2026-07-16 15:13:30.093933	2026-07-16 15:13:30.093933	corrupted isbn test	\N
582	Remove Authors Test	eng	0	novel		0	2026-07-16 15:13:30.521409	2026-07-16 15:13:30.521409	remove authors test	\N
583	Remove Genres Test	eng	0	novel		0	2026-07-16 15:13:30.538622	2026-07-16 15:13:30.538622	remove genres test	\N
584	Remove Tags Test	eng	0	novel		0	2026-07-16 15:13:30.556039	2026-07-16 15:13:30.556039	remove tags test	\N
585	Nil Authors Updated Title	eng	0	novel		0	2026-07-16 15:13:30.572756	2026-07-16 15:13:30.578931	nil authors updated title	\N
586	Add Author Test	eng	0	novel		0	2026-07-16 15:13:30.589395	2026-07-16 15:13:30.589395	add author test	\N
587	Test Book Part 1	eng	2023	novel	First test book created via API	0	2026-07-16 15:18:54.200887	2026-07-16 15:18:54.200887	test book part 1	\N
588	Test Book Part 2	eng	2024	novel	Second test book created via API	0	2026-07-16 15:18:54.206965	2026-07-16 15:18:54.206965	test book part 2	\N
589	Updated Book Title	eng	2022	novel	Updated description	0	2026-07-16 15:18:54.218422	2026-07-16 15:18:54.224025	updated book title	\N
591	Updated Title	eng	0	novel	New annotation text	0	2026-07-16 15:18:54.251429	2026-07-16 15:18:54.261133	updated title	\N
592	Updated Title Only	eng	0	novel		0	2026-07-16 15:18:54.272473	2026-07-16 15:18:54.278758	updated title only	\N
593	Updated Title Empty ISBN	eng	0	novel		0	2026-07-16 15:18:54.290027	2026-07-16 15:18:54.295768	updated title empty isbn	\N
594	Book One	eng	0	novel		0	2026-07-16 15:18:54.31107	2026-07-16 15:18:54.31107	book one	\N
595	Book Two	eng	0	novel		0	2026-07-16 15:18:54.316896	2026-07-16 15:18:54.316896	book two	\N
596	Original Book Title	eng	0	novel		0	2026-07-16 15:18:54.328862	2026-07-16 15:18:54.328862	original book title	\N
597	Updated Title	eng	0	novel		0	2026-07-16 15:18:54.346092	2026-07-16 15:18:54.35341	updated title	\N
598	Corrupted ISBN Test	eng	0	novel		0	2026-07-16 15:18:54.366598	2026-07-16 15:18:54.366598	corrupted isbn test	\N
599	Remove Authors Test	eng	0	novel		0	2026-07-16 15:18:54.800548	2026-07-16 15:18:54.800548	remove authors test	\N
600	Remove Genres Test	eng	0	novel		0	2026-07-16 15:18:54.820132	2026-07-16 15:18:54.820132	remove genres test	\N
601	Remove Tags Test	eng	0	novel		0	2026-07-16 15:18:54.83711	2026-07-16 15:18:54.83711	remove tags test	\N
602	Nil Authors Updated Title	eng	0	novel		0	2026-07-16 15:18:54.852558	2026-07-16 15:18:54.858614	nil authors updated title	\N
604	Test Book Part 1	eng	2023	novel	First test book created via API	0	2026-07-16 17:30:24.267049	2026-07-16 17:30:24.267049	test book part 1	\N
605	Test Book Part 2	eng	2024	novel	Second test book created via API	0	2026-07-16 17:30:24.272829	2026-07-16 17:30:24.272829	test book part 2	\N
606	Updated Book Title	eng	2022	novel	Updated description	0	2026-07-16 17:30:24.283475	2026-07-16 17:30:24.288888	updated book title	\N
608	Updated Title	eng	0	novel	New annotation text	0	2026-07-16 17:30:24.31887	2026-07-16 17:30:24.32835	updated title	\N
609	Updated Title Only	eng	0	novel		0	2026-07-16 17:30:24.340765	2026-07-16 17:30:24.346831	updated title only	\N
610	Updated Title Empty ISBN	eng	0	novel		0	2026-07-16 17:30:24.357448	2026-07-16 17:30:24.363481	updated title empty isbn	\N
611	Book One	eng	0	novel		0	2026-07-16 17:30:24.373031	2026-07-16 17:30:24.373031	book one	\N
612	Book Two	eng	0	novel		0	2026-07-16 17:30:24.378939	2026-07-16 17:30:24.378939	book two	\N
613	Original Book Title	eng	0	novel		0	2026-07-16 17:30:24.391772	2026-07-16 17:30:24.391772	original book title	\N
614	Updated Title	eng	0	novel		0	2026-07-16 17:30:24.40963	2026-07-16 17:30:24.415964	updated title	\N
615	Corrupted ISBN Test	eng	0	novel		0	2026-07-16 17:30:24.425615	2026-07-16 17:30:24.425615	corrupted isbn test	\N
616	Remove Authors Test	eng	0	novel		0	2026-07-16 17:30:24.852068	2026-07-16 17:30:24.852068	remove authors test	\N
617	Remove Genres Test	eng	0	novel		0	2026-07-16 17:30:24.868011	2026-07-16 17:30:24.868011	remove genres test	\N
618	Remove Tags Test	eng	0	novel		0	2026-07-16 17:30:24.883889	2026-07-16 17:30:24.883889	remove tags test	\N
619	Nil Authors Updated Title	eng	0	novel		0	2026-07-16 17:30:24.901372	2026-07-16 17:30:24.907731	nil authors updated title	\N
620	Add Author Test	eng	0	novel		0	2026-07-16 17:30:24.91708	2026-07-16 17:30:24.91708	add author test	\N
621	Test Book Part 1	eng	2023	novel	First test book created via API	0	2026-07-16 19:09:11.475866	2026-07-16 19:09:11.475866	test book part 1	\N
622	Test Book Part 2	eng	2024	novel	Second test book created via API	0	2026-07-16 19:09:11.482977	2026-07-16 19:09:11.482977	test book part 2	\N
874	Updated Title	eng	0	novel	New annotation text	0	2026-07-21 16:09:01.036043	2026-07-21 16:09:01.046451	updated title	\N
623	Updated Book Title	eng	2022	novel	Updated description	0	2026-07-16 19:09:11.493664	2026-07-16 19:09:11.499599	updated book title	\N
884	Remove Tags Test	eng	0	novel		0	2026-07-21 16:09:01.69528	2026-07-21 16:09:01.69528	remove tags test	\N
625	Updated Title	eng	0	novel	New annotation text	0	2026-07-16 19:09:11.531303	2026-07-16 19:09:11.541887	updated title	\N
626	Updated Title Only	eng	0	novel		0	2026-07-16 19:09:11.554085	2026-07-16 19:09:11.560079	updated title only	\N
627	Updated Title Empty ISBN	eng	0	novel		0	2026-07-16 19:09:11.569106	2026-07-16 19:09:11.574496	updated title empty isbn	\N
628	Book One	eng	0	novel		0	2026-07-16 19:09:11.584764	2026-07-16 19:09:11.584764	book one	\N
629	Book Two	eng	0	novel		0	2026-07-16 19:09:11.590444	2026-07-16 19:09:11.590444	book two	\N
630	Original Book Title	eng	0	novel		0	2026-07-16 19:09:11.602721	2026-07-16 19:09:11.602721	original book title	\N
631	Updated Title	eng	0	novel		0	2026-07-16 19:09:11.619578	2026-07-16 19:09:11.626434	updated title	\N
632	Corrupted ISBN Test	eng	0	novel		0	2026-07-16 19:09:11.636018	2026-07-16 19:09:11.636018	corrupted isbn test	\N
633	Remove Authors Test	eng	0	novel		0	2026-07-16 19:09:12.071822	2026-07-16 19:09:12.071822	remove authors test	\N
634	Remove Genres Test	eng	0	novel		0	2026-07-16 19:09:12.088964	2026-07-16 19:09:12.088964	remove genres test	\N
635	Remove Tags Test	eng	0	novel		0	2026-07-16 19:09:12.105906	2026-07-16 19:09:12.105906	remove tags test	\N
636	Nil Authors Updated Title	eng	0	novel		0	2026-07-16 19:09:12.124156	2026-07-16 19:09:12.130096	nil authors updated title	\N
637	Add Author Test	eng	0	novel		0	2026-07-16 19:09:12.139744	2026-07-16 19:09:12.139744	add author test	\N
638	Test Book Part 1	eng	2023	novel	First test book created via API	0	2026-07-17 21:27:15.047921	2026-07-17 21:27:15.047921	test book part 1	\N
639	Test Book Part 2	eng	2024	novel	Second test book created via API	0	2026-07-17 21:27:15.054215	2026-07-17 21:27:15.054215	test book part 2	\N
640	Updated Book Title	eng	2022	novel	Updated description	0	2026-07-17 21:27:15.065208	2026-07-17 21:27:15.070967	updated book title	\N
642	Updated Title	eng	0	novel	New annotation text	0	2026-07-17 21:27:15.094445	2026-07-17 21:27:15.104801	updated title	\N
643	Updated Title Only	eng	0	novel		0	2026-07-17 21:27:15.116195	2026-07-17 21:27:15.12221	updated title only	\N
644	Updated Title Empty ISBN	eng	0	novel		0	2026-07-17 21:27:15.1349	2026-07-17 21:27:15.140488	updated title empty isbn	\N
645	Book One	eng	0	novel		0	2026-07-17 21:27:15.148364	2026-07-17 21:27:15.148364	book one	\N
646	Book Two	eng	0	novel		0	2026-07-17 21:27:15.153879	2026-07-17 21:27:15.153879	book two	\N
647	Original Book Title	eng	0	novel		0	2026-07-17 21:27:15.163861	2026-07-17 21:27:15.163861	original book title	\N
648	Updated Title	eng	0	novel		0	2026-07-17 21:27:15.178931	2026-07-17 21:27:15.184927	updated title	\N
649	Corrupted ISBN Test	eng	0	novel		0	2026-07-17 21:27:15.19212	2026-07-17 21:27:15.19212	corrupted isbn test	\N
650	Remove Authors Test	eng	0	novel		0	2026-07-17 21:27:15.569617	2026-07-17 21:27:15.569617	remove authors test	\N
651	Remove Genres Test	eng	0	novel		0	2026-07-17 21:27:15.585243	2026-07-17 21:27:15.585243	remove genres test	\N
652	Remove Tags Test	eng	0	novel		0	2026-07-17 21:27:15.600539	2026-07-17 21:27:15.600539	remove tags test	\N
653	Nil Authors Updated Title	eng	0	novel		0	2026-07-17 21:27:15.616273	2026-07-17 21:27:15.62267	nil authors updated title	\N
654	Add Author Test	eng	0	novel		0	2026-07-17 21:27:15.631743	2026-07-17 21:27:15.631743	add author test	\N
655	Test Book Part 1	eng	2023	novel	First test book created via API	0	2026-07-18 09:12:30.470262	2026-07-18 09:12:30.470262	test book part 1	\N
656	Test Book Part 2	eng	2024	novel	Second test book created via API	0	2026-07-18 09:12:30.486615	2026-07-18 09:12:30.486615	test book part 2	\N
657	Updated Book Title	eng	2022	novel	Updated description	0	2026-07-18 09:12:30.497268	2026-07-18 09:12:30.502395	updated book title	\N
659	Updated Title	eng	0	novel	New annotation text	0	2026-07-18 09:12:30.539482	2026-07-18 09:12:30.549039	updated title	\N
660	Updated Title Only	eng	0	novel		0	2026-07-18 09:12:30.563162	2026-07-18 09:12:30.568486	updated title only	\N
661	Updated Title Empty ISBN	eng	0	novel		0	2026-07-18 09:12:30.580551	2026-07-18 09:12:30.585769	updated title empty isbn	\N
662	Book One	eng	0	novel		0	2026-07-18 09:12:30.597838	2026-07-18 09:12:30.597838	book one	\N
663	Book Two	eng	0	novel		0	2026-07-18 09:12:30.603254	2026-07-18 09:12:30.603254	book two	\N
664	Original Book Title	eng	0	novel		0	2026-07-18 09:12:30.615992	2026-07-18 09:12:30.615992	original book title	\N
665	Updated Title	eng	0	novel		0	2026-07-18 09:12:30.632468	2026-07-18 09:12:30.639034	updated title	\N
666	Corrupted ISBN Test	eng	0	novel		0	2026-07-18 09:12:30.649945	2026-07-18 09:12:30.649945	corrupted isbn test	\N
667	Remove Authors Test	eng	0	novel		0	2026-07-18 09:12:31.131251	2026-07-18 09:12:31.131251	remove authors test	\N
668	Remove Genres Test	eng	0	novel		0	2026-07-18 09:12:31.150943	2026-07-18 09:12:31.150943	remove genres test	\N
669	Remove Tags Test	eng	0	novel		0	2026-07-18 09:12:31.167339	2026-07-18 09:12:31.167339	remove tags test	\N
670	Nil Authors Updated Title	eng	0	novel		0	2026-07-18 09:12:31.186479	2026-07-18 09:12:31.191904	nil authors updated title	\N
672	Паразиты сознания	rus	\N	novel		\N	2026-07-20 10:38:30.159804	2026-07-20 10:38:30.159804	паразиты сознания	\N
673	КРАСНАЯ КНИГА	rus	2010	novel		\N	2026-07-20 12:09:39.385954	2026-07-20 12:09:39.385954	красная книга	\N
674	Test Book Part 1	eng	2023	novel	First test book created via API	0	2026-07-20 15:01:22.430491	2026-07-20 15:01:22.430491	test book part 1	\N
675	Test Book Part 2	eng	2024	novel	Second test book created via API	0	2026-07-20 15:01:22.436108	2026-07-20 15:01:22.436108	test book part 2	\N
676	Updated Book Title	eng	2022	novel	Updated description	0	2026-07-20 15:01:22.447129	2026-07-20 15:01:22.452506	updated book title	\N
724	Add Author Testd	eng	0	novel		0	2026-07-20 18:45:54.045814	2026-07-24 13:21:32.09767	add author testd	\N
678	Updated Title	eng	0	novel	New annotation text	0	2026-07-20 15:01:22.481121	2026-07-20 15:01:22.492263	updated title	\N
679	Updated Title Only	eng	0	novel		0	2026-07-20 15:01:22.506192	2026-07-20 15:01:22.511802	updated title only	\N
680	Updated Title Empty ISBN	eng	0	novel		0	2026-07-20 15:01:22.526038	2026-07-20 15:01:22.531221	updated title empty isbn	\N
681	Book One	eng	0	novel		0	2026-07-20 15:01:22.543141	2026-07-20 15:01:22.543141	book one	\N
682	Book Two	eng	0	novel		0	2026-07-20 15:01:22.548669	2026-07-20 15:01:22.548669	book two	\N
683	Original Book Title	eng	0	novel		0	2026-07-20 15:01:22.562157	2026-07-20 15:01:22.562157	original book title	\N
684	Updated Title	eng	0	novel		0	2026-07-20 15:01:22.582043	2026-07-20 15:01:22.588419	updated title	\N
685	Corrupted ISBN Test	eng	0	novel		0	2026-07-20 15:01:22.601465	2026-07-20 15:01:22.601465	corrupted isbn test	\N
686	Remove Authors Test	eng	0	novel		0	2026-07-20 15:01:23.10249	2026-07-20 15:01:23.10249	remove authors test	\N
687	Remove Genres Test	eng	0	novel		0	2026-07-20 15:01:23.120299	2026-07-20 15:01:23.120299	remove genres test	\N
688	Remove Tags Test	eng	0	novel		0	2026-07-20 15:01:23.13512	2026-07-20 15:01:23.13512	remove tags test	\N
689	Nil Authors Updated Title	eng	0	novel		0	2026-07-20 15:01:23.154087	2026-07-20 15:01:23.159548	nil authors updated title	\N
691	Test Book Part 1	eng	2023	novel	First test book created via API	0	2026-07-20 17:32:12.962153	2026-07-20 17:32:12.962153	test book part 1	\N
692	Test Book Part 2	eng	2024	novel	Second test book created via API	0	2026-07-20 17:32:12.967865	2026-07-20 17:32:12.967865	test book part 2	\N
693	Updated Book Title	eng	2022	novel	Updated description	0	2026-07-20 17:32:12.980678	2026-07-20 17:32:12.986489	updated book title	\N
695	Updated Title	eng	0	novel	New annotation text	0	2026-07-20 17:32:13.018689	2026-07-20 17:32:13.027798	updated title	\N
696	Updated Title Only	eng	0	novel		0	2026-07-20 17:32:13.044077	2026-07-20 17:32:13.050358	updated title only	\N
697	Updated Title Empty ISBN	eng	0	novel		0	2026-07-20 17:32:13.062034	2026-07-20 17:32:13.068275	updated title empty isbn	\N
698	Book One	eng	0	novel		0	2026-07-20 17:32:13.078359	2026-07-20 17:32:13.078359	book one	\N
699	Book Two	eng	0	novel		0	2026-07-20 17:32:13.083713	2026-07-20 17:32:13.083713	book two	\N
700	Original Book Title	eng	0	novel		0	2026-07-20 17:32:13.10142	2026-07-20 17:32:13.10142	original book title	\N
701	Updated Title	eng	0	novel		0	2026-07-20 17:32:13.118734	2026-07-20 17:32:13.125374	updated title	\N
702	Corrupted ISBN Test	eng	0	novel		0	2026-07-20 17:32:13.136817	2026-07-20 17:32:13.136817	corrupted isbn test	\N
703	Remove Authors Test	eng	0	novel		0	2026-07-20 17:32:13.612781	2026-07-20 17:32:13.612781	remove authors test	\N
704	Remove Genres Test	eng	0	novel		0	2026-07-20 17:32:13.63024	2026-07-20 17:32:13.63024	remove genres test	\N
705	Remove Tags Test	eng	0	novel		0	2026-07-20 17:32:13.648537	2026-07-20 17:32:13.648537	remove tags test	\N
706	Nil Authors Updated Title	eng	0	novel		0	2026-07-20 17:32:13.664145	2026-07-20 17:32:13.670568	nil authors updated title	\N
707	Add Author Test	eng	0	novel		0	2026-07-20 17:32:13.680968	2026-07-20 17:32:13.680968	add author test	\N
708	Test Book Part 1	eng	2023	novel	First test book created via API	0	2026-07-20 18:45:53.345029	2026-07-20 18:45:53.345029	test book part 1	\N
709	Test Book Part 2	eng	2024	novel	Second test book created via API	0	2026-07-20 18:45:53.351772	2026-07-20 18:45:53.351772	test book part 2	\N
710	Updated Book Title	eng	2022	novel	Updated description	0	2026-07-20 18:45:53.362712	2026-07-20 18:45:53.368835	updated book title	\N
712	Updated Title	eng	0	novel	New annotation text	0	2026-07-20 18:45:53.398766	2026-07-20 18:45:53.408305	updated title	\N
713	Updated Title Only	eng	0	novel		0	2026-07-20 18:45:53.420315	2026-07-20 18:45:53.426926	updated title only	\N
714	Updated Title Empty ISBN	eng	0	novel		0	2026-07-20 18:45:53.441716	2026-07-20 18:45:53.4475	updated title empty isbn	\N
715	Book One	eng	0	novel		0	2026-07-20 18:45:53.460612	2026-07-20 18:45:53.460612	book one	\N
716	Book Two	eng	0	novel		0	2026-07-20 18:45:53.465728	2026-07-20 18:45:53.465728	book two	\N
717	Original Book Title	eng	0	novel		0	2026-07-20 18:45:53.478085	2026-07-20 18:45:53.478085	original book title	\N
718	Updated Title	eng	0	novel		0	2026-07-20 18:45:53.500753	2026-07-20 18:45:53.506801	updated title	\N
719	Corrupted ISBN Test	eng	0	novel		0	2026-07-20 18:45:53.516949	2026-07-20 18:45:53.516949	corrupted isbn test	\N
720	Remove Authors Test	eng	0	novel		0	2026-07-20 18:45:53.974118	2026-07-20 18:45:53.974118	remove authors test	\N
721	Remove Genres Test	eng	0	novel		0	2026-07-20 18:45:53.994799	2026-07-20 18:45:53.994799	remove genres test	\N
722	Remove Tags Test	eng	0	novel		0	2026-07-20 18:45:54.013938	2026-07-20 18:45:54.013938	remove tags test	\N
723	Nil Authors Updated Title	eng	0	novel		0	2026-07-20 18:45:54.030519	2026-07-20 18:45:54.035958	nil authors updated title	\N
725	Test Book Part 1	eng	2023	novel	First test book created via API	0	2026-07-21 13:08:39.385614	2026-07-21 13:08:39.385614	test book part 1	\N
726	Test Book Part 2	eng	2024	novel	Second test book created via API	0	2026-07-21 13:08:39.396844	2026-07-21 13:08:39.396844	test book part 2	\N
727	Updated Book Title	eng	2022	novel	Updated description	0	2026-07-21 13:08:39.409464	2026-07-21 13:08:39.4162	updated book title	\N
729	Updated Title	eng	0	novel	New annotation text	0	2026-07-21 13:08:39.449785	2026-07-21 13:08:39.459356	updated title	\N
730	Updated Title Only	eng	0	novel		0	2026-07-21 13:08:39.471339	2026-07-21 13:08:39.47756	updated title only	\N
731	Updated Title Empty ISBN	eng	0	novel		0	2026-07-21 13:08:39.488401	2026-07-21 13:08:39.494056	updated title empty isbn	\N
732	Book One	eng	0	novel		0	2026-07-21 13:08:39.504794	2026-07-21 13:08:39.504794	book one	\N
733	Book Two	eng	0	novel		0	2026-07-21 13:08:39.510116	2026-07-21 13:08:39.510116	book two	\N
734	Original Book Title	eng	0	novel		0	2026-07-21 13:08:39.524471	2026-07-21 13:08:39.524471	original book title	\N
735	Updated Title	eng	0	novel		0	2026-07-21 13:08:39.541718	2026-07-21 13:08:39.548241	updated title	\N
736	Corrupted ISBN Test	eng	0	novel		0	2026-07-21 13:08:39.560093	2026-07-21 13:08:39.560093	corrupted isbn test	\N
737	Remove Authors Test	eng	0	novel		0	2026-07-21 13:08:40.01312	2026-07-21 13:08:40.01312	remove authors test	\N
738	Remove Genres Test	eng	0	novel		0	2026-07-21 13:08:40.030444	2026-07-21 13:08:40.030444	remove genres test	\N
739	Remove Tags Test	eng	0	novel		0	2026-07-21 13:08:40.048555	2026-07-21 13:08:40.048555	remove tags test	\N
740	Nil Authors Updated Title	eng	0	novel		0	2026-07-21 13:08:40.064763	2026-07-21 13:08:40.071	nil authors updated title	\N
741	Add Author Test	eng	0	novel		0	2026-07-21 13:08:40.083217	2026-07-21 13:08:40.083217	add author test	\N
742	Test Book Part 1	eng	2023	novel	First test book created via API	0	2026-07-21 14:59:09.716402	2026-07-21 14:59:09.716402	test book part 1	\N
743	Test Book Part 2	eng	2024	novel	Second test book created via API	0	2026-07-21 14:59:09.723218	2026-07-21 14:59:09.723218	test book part 2	\N
744	Updated Book Title	eng	2022	novel	Updated description	0	2026-07-21 14:59:09.734982	2026-07-21 14:59:09.740859	updated book title	\N
796	Жизнь внутри пузыря. Неформальное руководство менеджера по выживанию в инвестируемом проекте	rus	2007	novel	Тем, кто вблизи наблюдал чудовищный рост популярности Интернета в 1998–1999 годах, может показаться, что начиная с 2005 года история того интернет-пузыря повторяется еще раз. Так ли это — рассказывается в увлекательной бизнес-повести «Жизнь внутри пузыря». Что происходило тогда, можно было увидеть только изнутри самих компаний. Именно этим и замечательна эта книга. Игорь Ашманов приоткрывает завесу над тем, что творилось во время его работы в одном из крупнейших интернет-порталов российского Интернета в 1999–2001 годах. Книга будет интересна не только тем, кто работает в сфере IT, но и всем менеджерам, вынужденным выживать в инвестируемых проектах любого рода.	\N	2026-07-21 15:46:29.412558	2026-07-21 15:46:29.412558	жизнь внутри пузыря. неформальное руководство менеджера по выживанию в инвестируемом проекте	\N
797	Data Science from Scratch: First Principles with Python	eng	\N	novel		\N	2026-07-21 15:46:45.354403	2026-07-21 15:46:45.354403	data science from scratch: first principles with python	\N
746	Updated Title	eng	0	novel	New annotation text	0	2026-07-21 14:59:09.77278	2026-07-21 14:59:09.784428	updated title	\N
747	Updated Title Only	eng	0	novel		0	2026-07-21 14:59:09.797791	2026-07-21 14:59:09.803781	updated title only	\N
875	Updated Title Only	eng	0	novel		0	2026-07-21 16:09:01.060855	2026-07-21 16:09:01.067473	updated title only	\N
748	Updated Title Empty ISBN	eng	0	novel		0	2026-07-21 14:59:09.814749	2026-07-21 14:59:09.820803	updated title empty isbn	\N
749	Book One	eng	0	novel		0	2026-07-21 14:59:09.831287	2026-07-21 14:59:09.831287	book one	\N
750	Book Two	eng	0	novel		0	2026-07-21 14:59:09.837351	2026-07-21 14:59:09.837351	book two	\N
751	Original Book Title	eng	0	novel		0	2026-07-21 14:59:09.853609	2026-07-21 14:59:09.853609	original book title	\N
752	Updated Title	eng	0	novel		0	2026-07-21 14:59:09.874288	2026-07-21 14:59:09.882389	updated title	\N
753	Corrupted ISBN Test	eng	0	novel		0	2026-07-21 14:59:09.896072	2026-07-21 14:59:09.896072	corrupted isbn test	\N
754	Remove Authors Test	eng	0	novel		0	2026-07-21 14:59:10.375155	2026-07-21 14:59:10.375155	remove authors test	\N
755	Remove Genres Test	eng	0	novel		0	2026-07-21 14:59:10.391754	2026-07-21 14:59:10.391754	remove genres test	\N
756	Remove Tags Test	eng	0	novel		0	2026-07-21 14:59:10.41118	2026-07-21 14:59:10.41118	remove tags test	\N
885	Nil Authors Updated Title	eng	0	novel		0	2026-07-21 16:09:01.71826	2026-07-21 16:09:01.724214	nil authors updated title	\N
757	Nil Authors Updated Title	eng	0	novel		0	2026-07-21 14:59:10.428556	2026-07-21 14:59:10.435361	nil authors updated title	\N
758	Add Author Test	eng	0	novel		0	2026-07-21 14:59:10.443922	2026-07-21 14:59:10.443922	add author test	\N
759	Test Book Part 1	eng	2023	novel	First test book created via API	0	2026-07-21 15:40:16.044542	2026-07-21 15:40:16.044542	test book part 1	\N
760	Test Book Part 2	eng	2024	novel	Second test book created via API	0	2026-07-21 15:40:16.051771	2026-07-21 15:40:16.051771	test book part 2	\N
891	Updated Title	eng	0	novel	New annotation text	0	2026-07-21 20:55:27.733729	2026-07-21 20:55:27.744294	updated title	\N
761	Updated Book Title	eng	2022	novel	Updated description	0	2026-07-21 15:40:16.066138	2026-07-21 15:40:16.072596	updated book title	\N
894	Book One	eng	0	novel		0	2026-07-21 20:55:27.792904	2026-07-21 20:55:27.792904	book one	\N
895	Book Two	eng	0	novel		0	2026-07-21 20:55:27.799325	2026-07-21 20:55:27.799325	book two	\N
763	Updated Title	eng	0	novel	New annotation text	0	2026-07-21 15:40:16.112249	2026-07-21 15:40:16.123991	updated title	\N
897	Updated Title	eng	0	novel		0	2026-07-21 20:55:27.828743	2026-07-21 20:55:27.835468	updated title	\N
764	Updated Title Only	eng	0	novel		0	2026-07-21 15:40:16.138729	2026-07-21 15:40:16.144815	updated title only	\N
899	Remove Authors Test	eng	0	novel		0	2026-07-21 20:55:28.306723	2026-07-21 20:55:28.306723	remove authors test	\N
765	Updated Title Empty ISBN	eng	0	novel		0	2026-07-21 15:40:16.160175	2026-07-21 15:40:16.166304	updated title empty isbn	\N
766	Book One	eng	0	novel		0	2026-07-21 15:40:16.178137	2026-07-21 15:40:16.178137	book one	\N
767	Book Two	eng	0	novel		0	2026-07-21 15:40:16.184526	2026-07-21 15:40:16.184526	book two	\N
768	Original Book Title	eng	0	novel		0	2026-07-21 15:40:16.20389	2026-07-21 15:40:16.20389	original book title	\N
900	Remove Genres Test	eng	0	novel		0	2026-07-21 20:55:28.325034	2026-07-21 20:55:28.325034	remove genres test	\N
769	Updated Title	eng	0	novel		0	2026-07-21 15:40:16.22509	2026-07-21 15:40:16.232	updated title	\N
770	Corrupted ISBN Test	eng	0	novel		0	2026-07-21 15:40:16.244406	2026-07-21 15:40:16.244406	corrupted isbn test	\N
771	Remove Authors Test	eng	0	novel		0	2026-07-21 15:40:16.874415	2026-07-21 15:40:16.874415	remove authors test	\N
772	Remove Genres Test	eng	0	novel		0	2026-07-21 15:40:16.896279	2026-07-21 15:40:16.896279	remove genres test	\N
773	Remove Tags Test	eng	0	novel		0	2026-07-21 15:40:16.915469	2026-07-21 15:40:16.915469	remove tags test	\N
901	Remove Tags Test	eng	0	novel		0	2026-07-21 20:55:28.342165	2026-07-21 20:55:28.342165	remove tags test	\N
774	Nil Authors Updated Title	eng	0	novel		0	2026-07-21 15:40:16.941188	2026-07-21 15:40:16.948621	nil authors updated title	\N
775	Add Author Test	eng	0	novel		0	2026-07-21 15:40:16.960577	2026-07-21 15:40:16.960577	add author test	\N
903	Add Author Test	eng	0	novel		0	2026-07-21 20:55:28.374795	2026-07-21 20:55:28.374795	add author test	\N
798	Время – деньги!	rus	2013	novel	Дейл Карнеги сказал: «Если вы хотите получить превосходные советы о том, как обращаться с людьми, управлять самим собой и совершенствовать свои личные качества, прочтите автобиографию Бенджамина Франклина – одну из самых увлекательных историй жизни». Бенджамин Франклин – политический деятель, дипломат, учёный, изобретатель, журналист, издатель и масон. Один из лидеров войны за независимость США. Первый американец, ставший иностранным членом Российской академии наук. Его биография находится в лидерах скачивания в Интернете во всем мире и будет интересна тем, кто ищет новые идеи, интересуется историей и не стоит на месте. В книгу вошли знаменитые «Советы молодому торговцу».	\N	2026-07-21 15:46:45.395766	2026-07-21 15:46:45.395766	время – деньги!	\N
799	Git_для_профессионального_программиста.pdf	eng	\N	novel		\N	2026-07-21 15:46:45.536446	2026-07-21 15:46:45.536446	git_для_профессионального_программиста.pdf	\N
800	Ускорение: Совершенствование методов хозяйствования	rus	\N	novel	В книге, написанной видными учеными-экономистами и хозяйственными руководителями, в свете решений XXVII съезда КПСС, последующих Пленумов ЦК КПСС рассматриваются актуальные проблемы коренной перестройки хозяйственного механизма — совершенствование системы управления, планирования, ценообразования, финансово-кредитных отношений. Особое внимание уделено переходу объединений и отраслей промышленности на самофинансирование. Для работников предприятий, объединений, министерств, плановых и финансовых органов. Может быть использована в системе экономического образования.	\N	2026-07-21 15:46:45.572924	2026-07-21 15:46:45.572924	ускорение: совершенствование методов хозяйствования	\N
801	Тайная Доктрина: Синтез науки, религии и философии	eng	\N	novel		\N	2026-07-21 15:47:29.570287	2026-07-21 15:47:29.570287	тайная доктрина: синтез науки, религии и философии	\N
802	Тайная Доктрина. Синтез науки, религии и философии Том II. Антропогенезис	eng	\N	novel		\N	2026-07-21 15:48:18.549859	2026-07-21 15:48:18.549859	тайная доктрина. синтез науки, религии и философии том ii. антропогенезис	\N
803	HPB-TD3.DOC	eng	\N	novel		\N	2026-07-21 15:49:18.673356	2026-07-21 15:49:18.673356	hpb-td3.doc	\N
804	Текст книги, предоставленный через выделение "Эту книгу хорошо дополняют", является ссылкой на другие издания и не содержит информации о заглавии или авторе самой основной работы.	eng	\N	novel		\N	2026-07-21 15:49:38.65411	2026-07-21 15:49:38.65411	текст книги, предоставленный через выделение "эту книгу хорошо дополняют", является ссылкой на другие издания и не содержит информации о заглавии или авторе самой основной работы.	\N
805	Так говорил Заратустра	rus	1885	novel	«Великое светило! К чему свелось бы твое счастье, если б не было у тебя тех, кому ты светишь! В течение десяти лет подымалось ты к моей пещере: ты пресытилось бы своим светом и этой дорогою, если б не было меня, моего орла и моей змеи. Но мы каждое утро поджидали тебя, принимали от тебя преизбыток твой и благословляли тебя. Взгляни! Я пресытился своей мудростью, как пчела, собравшая слишком много меду; мне нужны руки, простертые ко мне. Я хотел бы одарять и наделять до тех пор, пока мудрые среди людей не стали бы опять радоваться безумству своему, а бедные – богатству своему. Для этого я должен спуститься вниз: как делаешь ты каждый вечер, окунаясь в море и неся свет свой на другую сторону мира, ты, богатейшее светило! Я должен, подобно тебе, закатиться , как называют это люди, к которым хочу я спуститься. Так благослови же меня, ты, спокойное око, без зависти взирающее даже на чрезмерно большое счастье! Благослови чашу, готовую пролиться, чтобы золотистая влага текла из нее и несла всюду отблеск твоей отрады! Взгляни, эта чаша хочет опять стать пустою, и Заратустра хочет опять стать человеком…»	\N	2026-07-21 15:49:38.704073	2026-07-21 15:49:38.704073	так говорил заратустра	\N
806	Алмазная Сутра	eng	\N	novel	Комментарии к Ваджрачхедика Праджняпарамита Сутре Гаутамы Будды Комментарии к беседам Бодхидхармы с учениками	\N	2026-07-21 15:50:20.035534	2026-07-21 15:50:20.035534	алмазная сутра	\N
807	Антикризиская программа	eng	2009	novel		\N	2026-07-21 15:50:20.050556	2026-07-21 15:50:20.050556	антикризиская программа	\N
808	Белый Лотос	eng	2009	novel	Лотос олицетворяет главный смысл санньясы. Лотос растет в озере, однако вода не касается его. Лотос символизирует саму основу твоего существа: ты живешь в этом мире, но остаешься свидетелем. Ты остаешься в этом мире, но не являешься его частью. Ты участвуешь в нем, но не являешься его частью. Ты в этом мире, но не от мира сего. Когда ты становишься безмолвным и отрешенным наблюдателем жизни... на тебя прольется дождь из белых лотосов. Великий Мастер Ошо использует слова удивительнейшего из будд, создателя дзэн Бодхидхармы, чтобы позволить нам прикоснуться к своему собственному, не менее удивительному пространству.	\N	2026-07-21 15:50:20.093723	2026-07-21 15:50:20.093723	белый лотос	\N
876	Updated Title Empty ISBN	eng	0	novel		0	2026-07-21 16:09:01.078846	2026-07-21 16:09:01.085334	updated title empty isbn	\N
886	Add Author Test	eng	0	novel		0	2026-07-21 16:09:01.734127	2026-07-21 16:09:01.734127	add author test	\N
892	Updated Title Only	eng	0	novel		0	2026-07-21 20:55:27.756819	2026-07-21 20:55:27.764912	updated title only	\N
896	Original Book Title	eng	0	novel		0	2026-07-21 20:55:27.811507	2026-07-21 20:55:27.811507	original book title	\N
898	Corrupted ISBN Test	eng	0	novel		0	2026-07-21 20:55:27.846384	2026-07-21 20:55:27.846384	corrupted isbn test	\N
809	Близость. Доверие к себе и другому.	eng	2009	novel	Отношения типа "Ударь-И-Убеги" становятся более и более распространенными в таком лишенном корней обществе, как западное, которое менее приковано к традиционным семейным структурам и в котором более приемлем случайный и легкомысленный секс. Но в то же время возникает подспудное ощущение, что чего-то не хватает. И это что-то - качество близости. Это качество имеет мало общего с физическим, хотя секс, несомненно, - одна из возможных дверей. Но для близости важнее готовность показывать наши глубочайшие чувства и уязвимые места в доверии к тому, что другой человек будет обращаться с ними бережно. В конечном счете, готовность пойти на риск близости должна быть укоренена во внутренней силе, которая знает, что, даже если другой человек останется закрытым, даже если доверие будет предано, мы не потерпим никакого непоправимого ущерба. Это руководство собрано из фрагментов бесед Ошо, в которых он мягко и сострадательно, шаг за шагом подводит нас к тому, что делает близость пугающей; учит, как столкнуться с этими причинами лицом к лицу, как выйти за их пределы и как развивать себя и отношения, оставляющие больше места открытости и доверию.	\N	2026-07-21 15:50:20.11649	2026-07-21 15:50:20.11649	близость. доверие к себе и другому.	\N
810	Будда: Пустота Сердца	eng	\N	novel	Ошо, также известный во всем мире под именем Бхагван Шри Раджниш, — просветленный Мастер нашего времени. Ошо означает «океанический, растворенный в океане». В этой книге он говорит о том, что внутри каждого человека уже живет Будда — необходимо лишь раскрыть Вселенную внутри себя. Ошо на примерах высказываний известных Мастеров Дзен показывает путь достижения состояния внутренней гармонии и счастья, вечности и бессмертия, свободы и просветления.	\N	2026-07-21 15:50:20.140947	2026-07-21 15:50:20.140947	будда: пустота сердца	\N
811	Горчичное зерно. Комментарии к пятому Евангелию от св. Фомы	eng	\N	novel	«Горчичное зерно» — толкование Евангелия от Фомы, и перед вами — самая удивительная книга из всех книг о Христе... «Горчичное зерно» — вовсе не милая евангельская история. На этих страницах посеяны горчичные зерна радикальной и необратимой революции, остается только заметить их ростки. Иначе с нами может произойти то же самое, что случилось, похоже, с первыми учениками Иисуса, которые превратили его слова в оправдание собственного невежества и источник кажущегося утешения — и тем самым утратили чудесную возможность пойти вслед за Учителем.	\N	2026-07-21 15:50:20.202047	2026-07-21 15:50:20.202047	горчичное зерно. комментарии к пятому евангелию от св. фомы	\N
812	Гусь снаружи	eng	2009	novel	Великий философский деятель, Рико, однажды попросил Нансена разъяснить ему старый коан о гусе в бутылке. «Если человек сажает гусенка в бутылку», - сказал Рико, - «и кормит его, пока тот не вырастет, как он сможет извлечь его наружу, не убив его или не разбив бутылку?» Нансен громко хлопнул в ладоши и закричал: «Рико!» «Да, Мастер», - сказал философ, вздрогнув. «Смотри», - сказал Нансен, - «гусь снаружи!»	\N	2026-07-21 15:50:20.232632	2026-07-21 15:50:20.232632	гусь снаружи	\N
813	В поисках Чудесного. Чакры, Кундалини и семь тел	eng	\N	novel	В этой книге Ошо говорит о самом «эзотерическом» предмете в абсолютно практичной и земной манере. В чем суть семи тел, энергии Кундалини и чем отличается жизнь после смерти и возможность следующего воплощения для человека, освоившего лишь свое физическое тело, от того, кто осознал второе, третье и т. д.? Что значит получить шактипат от Мастера? Что такое грэйс? Чем отличается движение энергии Кундалини в мужчин: и женщине? Каково значение Тантры, и как любящие могут помочь друг другу в движении от нижнего центра к высшему? Почему Динамическая медитация может спасти мир от безумия? Вновь и вновь Ошо напоминает нам, что поиск чудесного является внутренним поиском, предупреждает об опасности затеряться в другом, будь то любимый или Мастер.	\N	2026-07-21 15:50:20.275048	2026-07-21 15:50:20.275048	в поисках чудесного. чакры, кундалини и семь тел	\N
814	Алмазные россыпи	eng	2009	novel		\N	2026-07-21 15:50:20.286397	2026-07-21 15:50:20.286397	алмазные россыпи	\N
815	Руководство к своду знаний по управлению проектами (Руководство PMBOK). Редакция 2000 года	eng	\N	novel		\N	2026-07-21 15:51:00.123321	2026-07-21 15:51:00.123321	руководство к своду знаний по управлению проектами (руководство pmbok). редакция 2000 года	\N
816	Лампа Мафусаила, или Крайняя битва чекистов с масонами	rus	2016	novel	Как известно, сложное международное положение нашей страны объясняется острым конфликтом российского руководства с мировым масонством. Но мало кому понятны корни этого противостояния, его финансовая подоплека и оккультный смысл. Гибридный роман В. Пелевина срывает покровы молчания с этой тайны, попутно разъясняя в простой и доступной форме главные вопросы мировой политики, экономики, культуры и антропогенеза. В центре повествования – три поколения дворянской семьи Можайских, служащие Отчизне в XIX, XX и XXI веках.	\N	2026-07-21 15:51:00.182008	2026-07-21 15:51:00.182008	лампа мафусаила, или крайняя битва чекистов с масонами	\N
877	Book One	eng	0	novel		0	2026-07-21 16:09:01.094962	2026-07-21 16:09:01.094962	book one	\N
878	Book Two	eng	0	novel		0	2026-07-21 16:09:01.101245	2026-07-21 16:09:01.101245	book two	\N
817	Искусство легких касаний	rus	\N	novel	В чем связь между монстрами с крыши Нотр-Дама, самобытным мистическим путем России и трансгендерными уборными Северной Америки? Мы всего в шаге от решения этой мучительной загадки! Детективное расследование известного российского историка и плейбоя К.П. Голгофского посвящено химерам и гаргойлям — не просто украшениям готических соборов, а феноменам совершенно особого рода. Их использовали тайные общества древности. А что, если эстафету подхватили спецслужбы? Что, если античные боги живут не только в сериалах с нашего домашнего торрента? Можно ли встретить их в реальном мире? Нужны ли нам их услуги, а им — наши? И наконец, самый насущный вопрос современности: «столыпин, куда ж несешься ты? дай ответ. Не дает ответа…» В книге ответ есть, и довольно подробный.	\N	2026-07-21 15:51:00.258903	2026-07-21 15:51:00.258903	искусство легких касаний	\N
818	Непобедимое солнце	rus	2020	novel	Саша – продвинутая московская блондинка. Ей тридцатник, вируса на горизонте еще нет, и она уезжает в путешествие, обещанное ей на индийской горе Аруначале лично Шивой. Саша встретит историков-некроэмпатов, римских принцепсов, американских корпоративных анархистов, турецких филологов-суфиев, российских шестнадцатых референтов, кубинских тихарей и секс-работниц – и других интересных людей (и не только). Но самое главное, она прикоснется к тайне тайн – и увидит, откуда и как возникает то, что Илон Маск называет компьютерной симуляцией, а Святая Церковь – Мiром Божьим. Какой стала Саша после встречи с тайной, вы узнаете из книги. Какой стала тайна после встречи с Сашей, вы уже немного в курсе и так.	\N	2026-07-21 15:51:00.320256	2026-07-21 15:51:00.320256	непобедимое солнце	\N
819	iPhuck 10	rus	2017	novel	Порфирий Петрович – литературно-полицейский алгоритм. Он расследует преступления и одновременно пишет об этом детективные романы, зарабатывая средства для Полицейского Управления. Маруха Чо – искусствовед с большими деньгами и баба с яйцами по официальному гендеру. Ее специальность – так называемый «гипс», искусство первой четверти XXI века. Ей нужен помощник для анализа рынка. Им становится взятый в аренду Порфирий. «iPhuck 10» – самый дорогой любовный гаджет на рынке и одновременно самый знаменитый из 244 детективов Порфирия Петровича. Это настоящий шедевр алгоритмической полицейской прозы конца века – энциклопедический роман о будущем любви, искусства и всего остального. #cybersex, #gadgets, #искусственныйИнтеллект, #современноеИскусство, #детектив, #genderStudies, #триллер, #кудаВсеКатится, #содержитНецензурнуюБрань, #makinMovies, #тыПолюбитьЗаставилаСебяЧтобыПлеснутьМнеВДушуЧернымЯдом, #résistance Содержится ненормативная лексика	\N	2026-07-21 15:51:00.380568	2026-07-21 15:51:00.380568	iphuck 10	\N
820	Шум. Несовершенство человеческих суждений	rus	2021	novel	Два одинаково уважаемых врача могут поставить пациенту совершенно разные диагнозы. Два одинаково честных судьи – вынести абсолютно разные вердикты по одному делу. Два одинаково опытных специалиста по подбору персонала – выбрать на одну и ту же должность разных соискателей… Почему это происходит? От чего зависит? Могут ли на такие важные решения влиять время суток или день недели? Даниэль Канеман вместе с Оливье Сибони и Кассом Р. Санстейном раскроют секреты шума – посторонних влияний на наши суждения – во многих областях: от медицины до криминалистики, от экономического прогнозирования до юриспруденции, и, что еще важнее, научат, как его уменьшить, а значит, начать находить лучшие решения. В формате PDF A4 сохранен издательский макет.	\N	2026-07-21 15:51:00.455477	2026-07-21 15:51:00.455477	шум. несовершенство человеческих суждений	\N
821	Анна Каренина	rus	1878	novel	«Анна Каренина», один из самых знаменитых романов Льва Толстого, начинается ставшей афоризмом фразой: «Все счастливые семьи похожи друг на друга, каждая несчастливая семья несчастлива по-своему». Это книга о вечных ценностях: о любви, о вере, о семье, о человеческом достоинстве.	\N	2026-07-21 15:51:00.588068	2026-07-21 15:51:00.588068	анна каренина	\N
844	Все и вся	rus	\N	novel	Г. ГУРДЖИЕВ ВСЕ и ВСЯ Л о н д о н ОБРАЩЕНИЕ К ЧИТАТЕЛЮ Читатель, в твоих руках книга о необычайно глубокой системе Знания, и написана она человеком, масштабы которого и реальные возможности которого намного превосходят обычный уровень людей. В основе развивающих сил и космических откровений лежал, по словам Георгия Ивановича Гурджиева, "сознательный труд и сознательное страдание". ...	\N	2026-07-21 16:02:00.815672	2026-07-21 16:02:00.815672	все и вся	\N
845	Встречи с замечательными людьми	rus	2008	novel		\N	2026-07-21 16:02:00.862362	2026-07-21 16:02:00.862362	встречи с замечательными людьми	\N
822	Возвращение Синей Бороды	rus	2026	novel	НЕЗАКОННОЕ ПОТРЕБЛЕНИЕ НАРКОТИЧЕСКИХ СРЕДСТВ, ПСИХОТРОПНЫХ ВЕЩЕСТВ, ИХ АНАЛОГОВ ПРИЧИНЯЕТ ВРЕД ЗДОРОВЬЮ, ИХ НЕЗАКОННЫЙ ОБОРОТ ЗАПРЕЩЕН И ВЛЕЧЕТ УСТАНОВЛЕННУЮ ЗАКОНОДАТЕЛЬСТВОМ ОТВЕТСТВЕННОСТЬ Жиль де Рэ – сподвижник Жанны д’Арк, маршал Франции – и кровавый серийный убийца-маньяк. Римский император Тиберий – современник Иисуса Христа – и один из самых известных развратников в истории. Какое отношение имеют они к острову Эпштейна? Что такое реинкарнация на самом деле? Ответственны ли мы за совершенное в прошлых жизнях? Были ли это действительно мы? Кто такой Джеффри Эпштейн и чем занимались его гости на самом деле? В чем роль тайного общества «Pink Sunset» и его медиумов? Новое расследование Константина Голгофского срывает покровы с тайн зловещего острова и разоблачает чудовищное моральное падение мировых элит с радикально новой стороны. Но одновременно это и рискованный спуск в глубины собственной души… Секреты грабителей пирамид, гнев египетских богов, зверства левых активистов и преступления венценосных сластолюбцев, тайны квантовых прыжков в прошлое… На бонус – ценнейший практический навык: как одолеть непобедимый соблазн или укоренившуюся плохую привычку? Люди знали это три тысячи лет назад, но давно забыли. А мы вот помним.	\N	2026-07-21 15:51:00.640797	2026-07-21 15:51:00.640797	возвращение синей бороды	\N
824	Жизнь-в-сновидении (Посвящение в мир магов)	eng	\N	novel		\N	2026-07-21 15:51:39.683694	2026-07-21 15:51:39.683694	жизнь-в-сновидении (посвящение в мир магов)	\N
825	Договориться можно обо всем	rus	\N	novel	Книгу «Договориться можно обо всем» следует отнести к идеальным справочникам переговорщиков. В данной аудиокниге пошагово подается материал, касающийся всевозможных тактических приемов и стратегических подходов современного переговорного процесса. Слушателю предоставляется возможность скачать Гэвин Кеннеди - Договориться можно обо всем и узнать о всевозможных психологических ловушках, приемах, с помощью которых можно выручить очень многие ситуации. Вы прослушаете ряд заданий, которые требуют нестандартного мышления и не поддаются шаблонным и привычным решениям. Автор делает попытку в разрушении годами навязанных стереотипов и подчеркивает ошибки, характерные для современной коммуникации. Книга в первую очередь написана для тех, кто регулярно участвует в каких бы то ни было переговорах. А это управленцы и бизнесмены, предприниматели и менеджеры по продажам, специалисты спецслужб и политики.	\N	2026-07-21 15:51:39.729419	2026-07-21 15:51:39.729419	договориться можно обо всем	\N
826	Инвестируй в Себя: Разбуди в себе исполина	eng	\N	novel		\N	2026-07-21 15:52:16.550161	2026-07-21 15:52:16.550161	инвестируй в себя: разбуди в себе исполина	\N
827	Бизнес в стиле фанк: Капитал пляшет под дудку таланта (отрывки из книги)	eng	\N	novel		\N	2026-07-21 15:53:01.242092	2026-07-21 15:53:01.242092	бизнес в стиле фанк: капитал пляшет под дудку таланта (отрывки из книги)	\N
829	isis1.zip	eng	\N	novel		\N	2026-07-21 15:54:48.286164	2026-07-21 15:54:48.286164	isis1.zip	\N
830	Путешествие к центру Земли	eng	\N	novel		\N	2026-07-21 15:55:27.376747	2026-07-21 15:55:27.376747	путешествие к центру земли	\N
863	Колесо времени	rus	1998	novel	Итак, «Колесо времени», очевидно, итоговая книга Карлоса Кастанеды. Может быть, он все же напишет что-нибудь еще, но эта книга все равно будет итоговой. Так она задумана. И пусть концентрированная мудрость дона Хуана поможет вам на вашем пути. Эта книга пропитана Силой.	\N	2026-07-21 16:07:37.647815	2026-07-21 16:07:37.647815	колесо времени	\N
864	Виктор Пелевин и эффект Пустоты	rus	\N	novel		\N	2026-07-21 16:07:37.655156	2026-07-21 16:07:37.655156	виктор пелевин и эффект пустоты	\N
865	Empire V	rus	\N	novel		\N	2026-07-21 16:07:37.681712	2026-07-21 16:07:37.681712	empire v	\N
879	Original Book Title	eng	0	novel		0	2026-07-21 16:09:01.117888	2026-07-21 16:09:01.117888	original book title	\N
887	Test Book Part 1	eng	2023	novel	First test book created via API	0	2026-07-21 20:55:27.680959	2026-07-21 20:55:27.680959	test book part 1	\N
888	Test Book Part 2	eng	2024	novel	Second test book created via API	0	2026-07-21 20:55:27.687234	2026-07-21 20:55:27.687234	test book part 2	\N
828	Тайная доктрина. Синтез науки, религии и философии. Том I. Космогенезис	eng	\N	novel		\N	2026-07-21 15:53:48.025576	2026-07-25 15:58:07.12564	тайная доктрина. синтез науки, религии и философии. том i. космогенезис	\N
831	Контекст жизни. Как научиться управлять привычками, которые управляют нами	rus	2021	novel	Часто бывает так, что умный, трудолюбивый человек старается, но не может получить желаемую должность, увеличить доходы, найти любовь или реализовать мечту. Почему не всегда усилия ведут к результату и как добиться желаемого? Владимир Герасичев, Иван Маурах и Арсен Рябуха считают, что мы сами создаем себе барьеры на пути к успеху и виной тому наши когнитивные привычки. Авторы разбирают семь основных – быть правым, быть хорошим, не рисковать, контролировать, оценивать и обобщать, экономить время, находить объяснения и оправдания – и рассказывают, как их распознать и изменить. Так что эта книга – практический инструмент для расширения границ возможного и улучшения качества вашей жизни.	\N	2026-07-21 15:55:27.399981	2026-07-21 15:55:27.399981	контекст жизни. как научиться управлять привычками, которые управляют нами	\N
832	Магический переход. Путь женщины-воина	eng	\N	novel		\N	2026-07-21 15:56:12.463689	2026-07-21 15:56:12.463689	магический переход. путь женщины-воина	\N
833	Понедельник начинается в субботу	rus	1964	novel	Шедевр русской фантастики!!! Блистающие юмором истории младшего научного сотрудника Александра Привалова стали настольной книгой многих поколений российских читателей. И даже сейчас, десятилетия спустя, повесть «Понедельник начинается в субботу», давным-давно уже ставшая «народным достоянием» любителей отечественной научной фантастики, читается так же легко и с таким же наслаждением, как и многие годы назад!	\N	2026-07-21 15:56:12.504626	2026-07-21 15:56:12.504626	понедельник начинается в субботу	\N
834	SHABONO A True Adventure in the Remote and Magical Heart of the South American Jungle	eng	\N	novel		\N	2026-07-21 15:57:02.278284	2026-07-21 15:57:02.278284	shabono a true adventure in the remote and magical heart of the south american jungle	\N
835	Магические пассы. Практическая мудрость шаманов древней Мексики	eng	\N	novel		\N	2026-07-21 15:57:42.098163	2026-07-21 15:57:42.098163	магические пассы. практическая мудрость шаманов древней мексики	\N
836	Путь воина: Два года с доктором Хуаном Матусом - Сон ведьмы.	eng	\N	novel		\N	2026-07-21 15:58:23.448196	2026-07-21 15:58:23.448196	путь воина: два года с доктором хуаном матусом - сон ведьмы.	\N
837	Взгляды из реального мира. Записи бесед и лекций Гурджиева. Самонаблюдение.	eng	\N	novel		\N	2026-07-21 15:59:05.869285	2026-07-21 15:59:05.869285	взгляды из реального мира. записи бесед и лекций гурджиева. самонаблюдение.	\N
838	НФ: Альманах научной фантастики 35 (1991)	rus	\N	novel	В очередной сборник НФ входят произведения авторов, чья литературная судьба так или иначе связана со всесоюзными семинарами молодых фантастов в Малеевке и Дубултах. При всем разнообразии их творчества авторов объединяет обостренное внимание не к космической, а к земной «составляющей» научной фантастики. Рассчитан на широкий круг читателей. Грядет ли «Новая волна»?: От составителя (1991) // Автор: Евгений Войскунский Повести и рассказы // Автор: Евгений Войскунский Танцы мужчин (1989) // Автор: Владимир Покровский Клубника со сливками (1991) // Автор: Андрей Саломатов Страх (1991) // Автор: Алексей В. Андреев Отдай мою посадочную ногу! (1990) // Авторы: Евгений Лукин, Любовь Лукина До зимы еще полгода (1990) // Автор: Эдуард Геворкян Проблема верволка в Средней полосе [= Верволки средней полосы] (1991) // Автор: Виктор Пелевин Спи (1991) // Автор: Виктор Пелевин Синтез (1991) // Автор: Егор Лавров Публицистика // Автор: Евгений Войскунский Воспоминания о прошлом и немного о будущем (1991) // Автор: Владимир Гопман	\N	2026-07-21 15:59:05.914317	2026-07-21 15:59:05.914317	нф: альманах научной фантастики 35 (1991)	\N
839	Рассказы Вельзевула своему внуку; Объективно-беспристрастная критика жизни людей; Всё и Вся (в трех сериях)	eng	\N	novel		\N	2026-07-21 15:59:47.799375	2026-07-21 15:59:47.799375	рассказы вельзевула своему внуку; объективно-беспристрастная критика жизни людей; все и вся (в трех сериях)	\N
840	Шестое и последнее пребывание Вельзевула на поверхности Нашей Земли (часть из цикла романов "Архиепископ Плетенецкий")	eng	\N	novel		\N	2026-07-21 16:00:29.791841	2026-07-21 16:00:29.791841	шестое и последнее пребывание вельзевула на поверхности нашей земли (часть из цикла романов "архиепископ плетенецкий")	\N
841	Свет на Пути в Гору Святых, Великий Учитель Гаутам и Друзья, Книга 1	eng	\N	novel		\N	2026-07-21 16:01:07.795816	2026-07-21 16:01:07.795816	свет на пути в гору святых, великий учитель гаутам и друзья, книга 1	\N
842	Голос безмолвия, или Два пути; Семь врат (из сокровенных индусских писаний)	eng	\N	novel		\N	2026-07-21 16:02:00.708856	2026-07-21 16:02:00.708856	голос безмолвия, или два пути; семь врат (из сокровенных индусских писаний)	\N
843	Беседы с учениками	rus	\N	novel		\N	2026-07-21 16:02:00.721419	2026-07-21 16:02:00.721419	беседы с учениками	\N
880	Updated Title	eng	0	novel		0	2026-07-21 16:09:01.141347	2026-07-21 16:09:01.147765	updated title	\N
846	Жизнь реальна только тогда, когда "Я есть"	rus	\N	novel	В этой книге Гурджиев обращается к современному человеку, который уже не способен распознавать истину, открытую ему в различных формах с самых ранних времен, человеку, глубоко неудовлетворенному, чувствующему себя изолированным и ведущим бессмысленную жизнь. Перед читателем открывается метод действия Учителя, который своим присутствием обязывает прийти к окончательному решению, обязывает знать, чего человек хочет. Великий Мастер содействует созданию в мышлении и чувствах читателя истинного, не фантастического представления о мире, существующем в реальности, а не о том мире, который воспринимает каждый человек в данный момент. Для широкого круга читателей.	\N	2026-07-21 16:02:00.936861	2026-07-21 16:02:00.936861	жизнь реальна только тогда, когда "я есть"	\N
847	Закономерное разнообразие проявлений человеческой индивидуальности	rus	\N	novel		\N	2026-07-21 16:02:00.945292	2026-07-21 16:02:00.945292	закономерное разнообразие проявлений человеческой индивидуальности	\N
848	Последний час жизни	rus	\N	novel		\N	2026-07-21 16:02:00.952545	2026-07-21 16:02:00.952545	последний час жизни	\N
849	Человек - это многосложное существо	rus	2008	novel		\N	2026-07-21 16:02:00.975786	2026-07-21 16:02:00.975786	человек - это многосложное существо	\N
850	Эссе и размышления о Человеке и его Учении	rus	\N	novel		\N	2026-07-21 16:02:00.981325	2026-07-21 16:02:00.981325	эссе и размышления о человеке и его учении	\N
851	Дао дэ цзин	rus	\N	novel	В последний год жизни Лев Толстой интересовался религиозно-философской системой, созданной полулегендарным китайским философом Лао-цзы (его ещё называют Учителем Лао, Старым Ребёнком). Самая ранняя биография Лао-цзы обнаруживается в разделе «Жизнеописания» книги Сыма Цяня (II-I век до н.э.) «Исторические записки». Сыма Цянь даёт несколько вариантов биографии философа, называя его «благородным мужем-отшельником». Главный труд, который приписывается Лао-цзы, - «Дао дэ цзин» («Канон пути и благодати»), трактат, лёгший в основу даосизма. Толстому были близки многие идеи Лао-цзы - в частности, идея о мудром, святом и бездеятельном вожде («Кто любит народ и управляет им, тот должен быть бездеятельным»), а также его осуждение попыток насильственного изменения мира и человеческой природы. Кроме того, считается, что Лао-цзы критически относился к современному ему правительству («Оттого народ голодает, что слишком велики и тяжелы государственные налоги. Это именно причина бедствий народа»). Вне всякого сомнения, Толстой разделял эти взгляды Лао-цзы, хотя мистической стороны даосизма и обожествления Учителя Лао он, конечно, не принимал и не понимал. В данном, электронном виде китайского канона Лао-цзы, публикуется перевод известного даосского трактата «Дао дэ цзин», который выполнил отечественный китаевед Владимир Малявин, учитывая все новейшие научные данные, а также снабдил текст подробными примечаниями и комментариями. Для просмотра всех иллюстраций к изданию, рекомендуем знакомиться с книгой в формате fb2.	\N	2026-07-21 16:02:01.050855	2026-07-21 16:02:01.050855	дао дэ цзин	\N
852	Путь джедая	rus	2020	novel	Инструменты самонаблюдения и конструирования личного рецепта успеха от автора бестселлера «Джедайские техники»: мысли, однократные действия-«вакцины», регулярные практики и индикаторы, комбинируя которые можно найти свой уникальный подход.	\N	2026-07-21 16:02:01.162583	2026-07-21 16:02:01.162583	путь джедая	\N
853	Название и автор указанного текста не определены, так как представлены дополнительные книги.	eng	\N	novel		\N	2026-07-21 16:02:13.116777	2026-07-21 16:02:13.116777	название и автор указанного текста не определены, так как представлены дополнительные книги.	\N
854	Учение дона Хуана: путь знания индейцев яки	rus	\N	novel	Феномен Кастанеды невероятен. На нашей планете еще не было писателя, который так изменил бы взгляд человечества на привычный мир. Кто он был - Карлос Кастанеда? Ученик величайшего Учителя или величайший мистификатор? Да какое это имеет значение! Если старый Нагваль дон Хуан - его выдумка, то великий учитель - сам Кастанеда. Человек, сдвинувший точку сборки всего человечества Нет и, возможно, не будет на Земле книг, равных по Силе и тайне книгам Карлоса Кастанеды.	\N	2026-07-21 16:02:13.150713	2026-07-21 16:02:13.150713	учение дона хуана: путь знания индейцев яки	\N
855	ОТДЕЛЕННАЯ РЕАЛЬНОСТЬ (Книга 2)	eng	\N	novel		\N	2026-07-21 16:02:51.827047	2026-07-21 16:02:51.827047	отделенная реальность (книга 2)	\N
856	Путешествие в Икстлан. Путь к знанию и силе йоги шаманизма мексиканских индейцев	eng	\N	novel		\N	2026-07-21 16:03:37.246577	2026-07-21 16:03:37.246577	путешествие в икстлан. путь к знанию и силе йоги шаманизма мексиканских индейцев	\N
857	Сказки о силе: книга 4	eng	\N	novel		\N	2026-07-21 16:04:13.680014	2026-07-21 16:04:13.680014	сказки о силе: книга 4	\N
858	Второе кольцо силы. Перекрестье жизней  \n(Note: The book mentioned is part 5 of a larger work, the full title may vary in different editions.)	eng	\N	novel		\N	2026-07-21 16:05:01.023192	2026-07-21 16:05:01.023192	второе кольцо силы. перекрестье жизней  \n(note: the book mentioned is part 5 of a larger work, the full title may vary in different editions.)	\N
859	Книга 6. Дар Орла	eng	\N	novel		\N	2026-07-21 16:05:34.764247	2026-07-21 16:05:34.764247	книга 6. дар орла	\N
860	Внутренний огонь. Книга семь: применение левого ядра в повседневной жизни	eng	\N	novel		\N	2026-07-21 16:06:12.236656	2026-07-21 16:06:12.236656	внутренний огонь. книга семь: применение левого ядра в повседневной жизни	\N
861	СилаБезмолвия. Пролог	eng	\N	novel		\N	2026-07-21 16:06:45.91521	2026-07-21 16:06:45.91521	силабезмолвия. пролог	\N
862	Активная сторона бесконечности. Том 10. Книги о мастерстве.	eng	\N	novel		\N	2026-07-21 16:07:37.630942	2026-07-21 16:07:37.630942	активная сторона бесконечности. том 10. книги о мастерстве.	\N
867	Relics. Раннее и неизданное (Сборник)	rus	\N	novel	«Relics. Раннее и неизданное» — сборник ранних произведений автора. Пелевин как всегда оригинален — не только в своем творчестве, но и в его преподнесении читателю: содержание книги повторяет хронологию событий, произошедших с Россией на стыке XX и XXI веков. Мы достаточно удалились от девяностых, чтобы разглядеть их без эффекта «лицом к лицу лица не увидать». Поэтому в наши дни выходят фильмы-медитации вроде «Жмурок». "Relics — своего рода «жмурки духа», ностальгическое воспоминание о времени малиновых пиджаков в суровую эпоху оранжевых галстуков. Книга составлена таким образом, чтобы ее содержание примерно соответствовало хронологии событий: как мы туда прибыли (для этого включены несколько совсем ранних рассказов), во что вляпались и куда после этого делись… Эти тексты лежали не «в столе» — они публиковались в журналах и газетах, просто большая их часть не выходила в книжном формате. Многие гуляют по интернету в изувеченном виде, лучше издать их, наконец, по-человечески. Кроме того, там есть эссе, которые не печатались в России. А название я позаимствовал у своей любимой пластинки Pink Floyd." В.О.Пелевин В сборник вошли произведения: Психическая атака сонет • Колдун Игнат и люди • СССР Тайшоу Чжуань • Жизнь и приключения сарая номер XII • Водонапорная башня • Миттельшпиль • Музыка со столба • Откровение Крегера • Оружие возмездия • Бубен нижнего мира • Краткая история пэйнтбола в Москве • Нижняя тундра • Святочный киберпанк, или Рождественская ночь-117.DIR • Time out • Греческий вариант • Who by fire • Икстлан — Петушки • ГКЧП как тетраграмматон • Зомбификация Опыт сравнительной антропологии • Джон Фаулз и трагедия русского либерализма • Имена олигархов на карте Родины • Мост, который я хотел перейти	\N	2026-07-21 16:07:37.758131	2026-07-21 16:07:37.758131	relics. раннее и неизданное (сборник)	\N
868	S.N.U.F.F.	rus	2011	novel	Роман-утопия Виктора Пелевина о глубочайших тайнах женского сердца и высших секретах летного мастерства.	\N	2026-07-21 16:07:37.817524	2026-07-21 16:07:37.817524	s.n.u.f.f.	\N
869	Timeout, или Вечерняя Москва	rus	\N	novel		\N	2026-07-21 16:07:37.826585	2026-07-21 16:07:37.826585	timeout, или вечерняя москва	\N
902	Nil Authors Updated Title	eng	0	novel		0	2026-07-21 20:55:28.358562	2026-07-21 20:55:28.364815	nil authors updated title	\N
904	Test Book Part 1	eng	2023	novel	First test book created via API	0	2026-07-21 21:23:05.812351	2026-07-21 21:23:05.812351	test book part 1	\N
905	Test Book Part 2	eng	2024	novel	Second test book created via API	0	2026-07-21 21:23:05.818941	2026-07-21 21:23:05.818941	test book part 2	\N
906	Updated Book Title	eng	2022	novel	Updated description	0	2026-07-21 21:23:05.83424	2026-07-21 21:23:05.840803	updated book title	\N
908	Updated Title	eng	0	novel	New annotation text	0	2026-07-21 21:23:05.872347	2026-07-21 21:23:05.883946	updated title	\N
909	Updated Title Only	eng	0	novel		0	2026-07-21 21:23:05.89653	2026-07-21 21:23:05.902345	updated title only	\N
910	Updated Title Empty ISBN	eng	0	novel		0	2026-07-21 21:23:05.91756	2026-07-21 21:23:05.92355	updated title empty isbn	\N
911	Book One	eng	0	novel		0	2026-07-21 21:23:05.93518	2026-07-21 21:23:05.93518	book one	\N
912	Book Two	eng	0	novel		0	2026-07-21 21:23:05.941308	2026-07-21 21:23:05.941308	book two	\N
913	Original Book Title	eng	0	novel		0	2026-07-21 21:23:05.955498	2026-07-21 21:23:05.955498	original book title	\N
914	Updated Title	eng	0	novel		0	2026-07-21 21:23:05.974623	2026-07-21 21:23:05.981169	updated title	\N
915	Corrupted ISBN Test	eng	0	novel		0	2026-07-21 21:23:05.991554	2026-07-21 21:23:05.991554	corrupted isbn test	\N
916	Remove Authors Test	eng	0	novel		0	2026-07-21 21:23:06.482959	2026-07-21 21:23:06.482959	remove authors test	\N
917	Remove Genres Test	eng	0	novel		0	2026-07-21 21:23:06.501982	2026-07-21 21:23:06.501982	remove genres test	\N
918	Remove Tags Test	eng	0	novel		0	2026-07-21 21:23:06.520624	2026-07-21 21:23:06.520624	remove tags test	\N
919	Nil Authors Updated Title	eng	0	novel		0	2026-07-21 21:23:06.538302	2026-07-21 21:23:06.544099	nil authors updated title	\N
920	Add Author Test	eng	32442	novel		0	2026-07-21 21:23:06.552965	2026-07-22 09:29:02.893456	add author test	\N
200	Updated Title	eng	3	novel	New annotation text	0	2026-07-12 12:18:02.548976	2026-07-22 09:29:12.958949	updated title	\N
921	Test Book Part 1	eng	2023	novel	First test book created via API	0	2026-07-22 09:33:47.713172	2026-07-22 09:33:47.713172	test book part 1	\N
922	Test Book Part 2	eng	2024	novel	Second test book created via API	0	2026-07-22 09:33:47.719627	2026-07-22 09:33:47.719627	test book part 2	\N
923	Updated Book Title	eng	2022	novel	Updated description	0	2026-07-22 09:33:47.73352	2026-07-22 09:33:47.740717	updated book title	\N
925	Updated Title	eng	0	novel	New annotation text	0	2026-07-22 09:33:47.774321	2026-07-22 09:33:47.784246	updated title	\N
926	Updated Title Only	eng	0	novel		0	2026-07-22 09:33:47.79992	2026-07-22 09:33:47.806235	updated title only	\N
927	Updated Title Empty ISBN	eng	0	novel		0	2026-07-22 09:33:47.82124	2026-07-22 09:33:47.827384	updated title empty isbn	\N
928	Book One	eng	0	novel		0	2026-07-22 09:33:47.840991	2026-07-22 09:33:47.840991	book one	\N
929	Book Two	eng	0	novel		0	2026-07-22 09:33:47.847151	2026-07-22 09:33:47.847151	book two	\N
930	Original Book Title	eng	0	novel		0	2026-07-22 09:33:47.863741	2026-07-22 09:33:47.863741	original book title	\N
931	Updated Title	eng	0	novel		0	2026-07-22 09:33:47.888197	2026-07-22 09:33:47.894712	updated title	\N
932	Corrupted ISBN Test	eng	0	novel		0	2026-07-22 09:33:52.900853	2026-07-22 09:33:52.900853	corrupted isbn test	\N
933	Remove Authors Test	eng	0	novel		0	2026-07-22 09:33:53.384278	2026-07-22 09:33:53.384278	remove authors test	\N
934	Remove Genres Test	eng	0	novel		0	2026-07-22 09:33:53.402597	2026-07-22 09:33:53.402597	remove genres test	\N
935	Remove Tags Test	eng	0	novel		0	2026-07-22 09:33:53.426572	2026-07-22 09:33:53.426572	remove tags test	\N
936	Nil Authors Updated Title	eng	0	novel		0	2026-07-22 09:33:53.443721	2026-07-22 09:33:53.450592	nil authors updated title	\N
937	Add Author Test	eng	0	novel		0	2026-07-22 09:33:53.460587	2026-07-22 09:33:53.460587	add author test	\N
938	Детский мир (сборник)	rus	2015	novel	Татьяна Толстая и Виктор Пелевин, Людмила Улицкая и Михаил Веллер, Захар Прилепин и Марина Степнова, Майя Кучерская и Людмила Петрушевская, Андрей Макаревич, Евгений Водолазкин, Александр Терехов и другие известные прозаики рассказывают в этом сборнике о пугающем детском опыте, в том числе – о своем личном. Эти рассказы уверенно разрушают миф о «розовом детстве»: первая любовь трагична, падать больно, жить, когда ты лишен опыта и знаний, страшно. Детство все воспринимает в полный рост, абсолютно всерьез, и потому проза о детстве обязана быть предельно серьезной – такой, как на страницах «Детского мира».	\N	2026-07-23 07:07:55.587012	2026-07-23 07:07:55.587012	детский мир (сборник)	\N
939	Test Book Part 1	eng	2023	novel	First test book created via API	0	2026-07-23 07:17:39.553704	2026-07-23 07:17:39.553704	test book part 1	\N
940	Test Book Part 2	eng	2024	novel	Second test book created via API	0	2026-07-23 07:17:39.561426	2026-07-23 07:17:39.561426	test book part 2	\N
941	Updated Book Title	eng	2022	novel	Updated description	0	2026-07-23 07:17:39.577495	2026-07-23 07:17:39.584045	updated book title	\N
943	Updated Title	eng	0	novel	New annotation text	0	2026-07-23 07:17:39.613987	2026-07-23 07:17:39.626287	updated title	\N
944	Updated Title Only	eng	0	novel		0	2026-07-23 07:17:39.63965	2026-07-23 07:17:39.646153	updated title only	\N
945	Updated Title Empty ISBN	eng	0	novel		0	2026-07-23 07:17:39.658919	2026-07-23 07:17:39.665165	updated title empty isbn	\N
946	Book One	eng	0	novel		0	2026-07-23 07:17:39.675216	2026-07-23 07:17:39.675216	book one	\N
947	Book Two	eng	0	novel		0	2026-07-23 07:17:39.681092	2026-07-23 07:17:39.681092	book two	\N
948	Original Book Title	eng	0	novel		0	2026-07-23 07:17:39.694974	2026-07-23 07:17:39.694974	original book title	\N
949	Updated Title	eng	0	novel		0	2026-07-23 07:17:39.716267	2026-07-23 07:17:39.722903	updated title	\N
950	Corrupted ISBN Test	eng	0	novel		0	2026-07-23 07:17:39.733793	2026-07-23 07:17:39.733793	corrupted isbn test	\N
951	Remove Authors Test	eng	0	novel		0	2026-07-23 07:17:40.237189	2026-07-23 07:17:40.237189	remove authors test	\N
952	Remove Genres Test	eng	0	novel		0	2026-07-23 07:17:40.254592	2026-07-23 07:17:40.254592	remove genres test	\N
953	Remove Tags Test	eng	0	novel		0	2026-07-23 07:17:40.272656	2026-07-23 07:17:40.272656	remove tags test	\N
954	Nil Authors Updated Title	eng	0	novel		0	2026-07-23 07:17:40.289769	2026-07-23 07:17:40.296694	nil authors updated title	\N
955	Add Author Test	eng	0	novel		0	2026-07-23 07:17:40.306254	2026-07-23 07:17:40.306254	add author test	\N
956	Test Book Part 1	eng	2023	novel	First test book created via API	0	2026-07-24 13:13:24.566758	2026-07-24 13:13:24.566758	test book part 1	\N
957	Test Book Part 2	eng	2024	novel	Second test book created via API	0	2026-07-24 13:13:24.573495	2026-07-24 13:13:24.573495	test book part 2	\N
958	Updated Book Title	eng	2022	novel	Updated description	0	2026-07-24 13:13:24.58461	2026-07-24 13:13:24.590634	updated book title	\N
960	Updated Title	eng	0	novel	New annotation text	0	2026-07-24 13:13:24.622676	2026-07-24 13:13:24.633272	updated title	\N
961	Updated Title Only	eng	0	novel		0	2026-07-24 13:13:24.646203	2026-07-24 13:13:24.652335	updated title only	\N
962	Updated Title Empty ISBN	eng	0	novel		0	2026-07-24 13:13:24.872152	2026-07-24 13:13:24.878426	updated title empty isbn	\N
963	Book One	eng	0	novel		0	2026-07-24 13:13:24.888625	2026-07-24 13:13:24.888625	book one	\N
964	Book Two	eng	0	novel		0	2026-07-24 13:13:24.895048	2026-07-24 13:13:24.895048	book two	\N
965	Original Book Title	eng	0	novel		0	2026-07-24 13:13:24.908016	2026-07-24 13:13:24.908016	original book title	\N
966	Updated Title	eng	0	novel		0	2026-07-24 13:13:24.926932	2026-07-24 13:13:24.933382	updated title	\N
967	Corrupted ISBN Test	eng	0	novel		0	2026-07-24 13:13:24.943588	2026-07-24 13:13:24.943588	corrupted isbn test	\N
968	Remove Authors Test	eng	0	novel		0	2026-07-24 13:13:25.455177	2026-07-24 13:13:25.455177	remove authors test	\N
969	Remove Genres Test	eng	0	novel		0	2026-07-24 13:13:25.473525	2026-07-24 13:13:25.473525	remove genres test	\N
970	Remove Tags Test	eng	0	novel		0	2026-07-24 13:13:25.490529	2026-07-24 13:13:25.490529	remove tags test	\N
971	Nil Authors Updated Title	eng	0	novel		0	2026-07-24 13:13:25.509177	2026-07-24 13:13:25.516836	nil authors updated title	\N
972	Add Author Test	eng	0	novel		0	2026-07-24 13:13:25.526854	2026-07-24 13:13:25.526854	add author test	\N
973	Test Book Part 1	eng	2023	novel	First test book created via API	0	2026-07-24 13:18:50.692058	2026-07-24 13:18:50.692058	test book part 1	\N
974	Test Book Part 2	eng	2024	novel	Second test book created via API	0	2026-07-24 13:18:50.698837	2026-07-24 13:18:50.698837	test book part 2	\N
975	Updated Book Title	eng	2022	novel	Updated description	0	2026-07-24 13:18:50.711429	2026-07-24 13:18:50.718905	updated book title	\N
977	Updated Title	eng	0	novel	New annotation text	0	2026-07-24 13:18:50.751683	2026-07-24 13:18:50.761733	updated title	\N
978	Updated Title Only	eng	0	novel		0	2026-07-24 13:18:50.777676	2026-07-24 13:18:50.784341	updated title only	\N
979	Updated Title Empty ISBN	eng	0	novel		0	2026-07-24 13:18:50.798002	2026-07-24 13:18:50.804009	updated title empty isbn	\N
980	Book One	eng	0	novel		0	2026-07-24 13:18:50.815429	2026-07-24 13:18:50.815429	book one	\N
981	Book Two	eng	0	novel		0	2026-07-24 13:18:50.822616	2026-07-24 13:18:50.822616	book two	\N
982	Original Book Title	eng	0	novel		0	2026-07-24 13:18:50.839801	2026-07-24 13:18:50.839801	original book title	\N
983	Updated Title	eng	0	novel		0	2026-07-24 13:18:50.859227	2026-07-24 13:18:50.865924	updated title	\N
984	Corrupted ISBN Test	eng	0	novel		0	2026-07-24 13:18:50.879886	2026-07-24 13:18:50.879886	corrupted isbn test	\N
985	Remove Authors Test	eng	0	novel		0	2026-07-24 13:18:51.419246	2026-07-24 13:18:51.419246	remove authors test	\N
986	Remove Genres Test	eng	0	novel		0	2026-07-24 13:18:51.441838	2026-07-24 13:18:51.441838	remove genres test	\N
987	Remove Tags Test	eng	0	novel		0	2026-07-24 13:18:51.460151	2026-07-24 13:18:51.460151	remove tags test	\N
988	Nil Authors Updated Title	eng	0	novel		0	2026-07-24 13:18:51.476998	2026-07-24 13:18:51.483782	nil authors updated title	\N
989	Add Author Test	eng	0	novel		0	2026-07-24 13:18:51.496996	2026-07-24 13:18:51.496996	add author test	\N
990	Test Book Part 1	eng	2023	novel	First test book created via API	0	2026-07-24 13:36:44.535594	2026-07-24 13:36:44.535594	test book part 1	\N
991	Test Book Part 2	eng	2024	novel	Second test book created via API	0	2026-07-24 13:36:44.541781	2026-07-24 13:36:44.541781	test book part 2	\N
992	Updated Book Title	eng	2022	novel	Updated description	0	2026-07-24 13:36:44.552856	2026-07-24 13:36:44.558921	updated book title	\N
994	Updated Title	eng	0	novel	New annotation text	0	2026-07-24 13:36:44.586251	2026-07-24 13:36:44.597046	updated title	\N
995	Updated Title Only	eng	0	novel		0	2026-07-24 13:36:44.610108	2026-07-24 13:36:44.617029	updated title only	\N
996	Updated Title Empty ISBN	eng	0	novel		0	2026-07-24 13:36:44.628097	2026-07-24 13:36:44.633855	updated title empty isbn	\N
997	Book One	eng	0	novel		0	2026-07-24 13:36:44.645743	2026-07-24 13:36:44.645743	book one	\N
998	Book Two	eng	0	novel		0	2026-07-24 13:36:44.651616	2026-07-24 13:36:44.651616	book two	\N
999	Original Book Title	eng	0	novel		0	2026-07-24 13:36:44.667209	2026-07-24 13:36:44.667209	original book title	\N
1000	Updated Title	eng	0	novel		0	2026-07-24 13:36:44.685492	2026-07-24 13:36:44.692938	updated title	\N
1001	Corrupted ISBN Test	eng	0	novel		0	2026-07-24 13:36:44.702462	2026-07-24 13:36:44.702462	corrupted isbn test	\N
1002	Remove Authors Test	eng	0	novel		0	2026-07-24 13:36:45.194849	2026-07-24 13:36:45.194849	remove authors test	\N
1003	Remove Genres Test	eng	0	novel		0	2026-07-24 13:36:45.212671	2026-07-24 13:36:45.212671	remove genres test	\N
1004	Remove Tags Test	eng	0	novel		0	2026-07-24 13:36:45.228532	2026-07-24 13:36:45.228532	remove tags test	\N
1005	Nil Authors Updated Title	eng	0	novel		0	2026-07-24 13:36:45.24484	2026-07-24 13:36:45.25148	nil authors updated title	\N
1006	Add Author Test	eng	0	novel		0	2026-07-24 13:36:45.26059	2026-07-24 13:36:45.26059	add author test	\N
1007	Test Book Part 1	eng	2023	novel	First test book created via API	0	2026-07-24 15:09:43.910989	2026-07-24 15:09:43.910989	test book part 1	\N
1008	Test Book Part 2	eng	2024	novel	Second test book created via API	0	2026-07-24 15:09:43.917654	2026-07-24 15:09:43.917654	test book part 2	\N
1009	Updated Book Title	eng	2022	novel	Updated description	0	2026-07-24 15:09:43.931523	2026-07-24 15:09:43.937495	updated book title	\N
1011	Updated Title	eng	0	novel	New annotation text	0	2026-07-24 15:09:43.965619	2026-07-24 15:09:43.977274	updated title	\N
1012	Updated Title Only	eng	0	novel		0	2026-07-24 15:09:43.991359	2026-07-24 15:09:43.997717	updated title only	\N
1013	Updated Title Empty ISBN	eng	0	novel		0	2026-07-24 15:09:44.012588	2026-07-24 15:09:44.018953	updated title empty isbn	\N
1014	Book One	eng	0	novel		0	2026-07-24 15:09:44.032559	2026-07-24 15:09:44.032559	book one	\N
1015	Book Two	eng	0	novel		0	2026-07-24 15:09:44.038459	2026-07-24 15:09:44.038459	book two	\N
1016	Original Book Title	eng	0	novel		0	2026-07-24 15:09:44.053328	2026-07-24 15:09:44.053328	original book title	\N
1017	Updated Title	eng	0	novel		0	2026-07-24 15:09:44.072821	2026-07-24 15:09:44.079294	updated title	\N
1018	Corrupted ISBN Test	eng	0	novel		0	2026-07-24 15:09:44.089424	2026-07-24 15:09:44.089424	corrupted isbn test	\N
1019	Remove Authors Test	eng	0	novel		0	2026-07-24 15:09:44.619403	2026-07-24 15:09:44.619403	remove authors test	\N
1020	Remove Genres Test	eng	0	novel		0	2026-07-24 15:09:44.638413	2026-07-24 15:09:44.638413	remove genres test	\N
1021	Remove Tags Test	eng	0	novel		0	2026-07-24 15:09:44.655964	2026-07-24 15:09:44.655964	remove tags test	\N
1022	Nil Authors Updated Title	eng	0	novel		0	2026-07-24 15:09:44.672235	2026-07-24 15:09:44.678575	nil authors updated title	\N
1023	Add Author Test	eng	0	novel		0	2026-07-24 15:09:44.68828	2026-07-24 15:09:44.68828	add author test	\N
1024	Test Book Part 1	eng	2023	novel	First test book created via API	0	2026-07-24 15:41:19.686104	2026-07-24 15:41:19.686104	test book part 1	\N
1025	Test Book Part 2	eng	2024	novel	Second test book created via API	0	2026-07-24 15:41:19.692653	2026-07-24 15:41:19.692653	test book part 2	\N
1026	Updated Book Title	eng	2022	novel	Updated description	0	2026-07-24 15:41:19.707093	2026-07-24 15:41:19.713372	updated book title	\N
1028	Updated Title	eng	0	novel	New annotation text	0	2026-07-24 15:41:19.742577	2026-07-24 15:41:19.753004	updated title	\N
1029	Updated Title Only	eng	0	novel		0	2026-07-24 15:41:19.767177	2026-07-24 15:41:19.77359	updated title only	\N
1030	Updated Title Empty ISBN	eng	0	novel		0	2026-07-24 15:41:19.78451	2026-07-24 15:41:19.790631	updated title empty isbn	\N
1031	Book One	eng	0	novel		0	2026-07-24 15:41:19.800165	2026-07-24 15:41:19.800165	book one	\N
1032	Book Two	eng	0	novel		0	2026-07-24 15:41:19.806294	2026-07-24 15:41:19.806294	book two	\N
1033	Original Book Title	eng	0	novel		0	2026-07-24 15:41:19.818826	2026-07-24 15:41:19.818826	original book title	\N
1034	Updated Title	eng	0	novel		0	2026-07-24 15:41:19.836339	2026-07-24 15:41:19.843678	updated title	\N
1035	Corrupted ISBN Test	eng	0	novel		0	2026-07-24 15:41:19.853324	2026-07-24 15:41:19.853324	corrupted isbn test	\N
1036	Remove Authors Test	eng	0	novel		0	2026-07-24 15:41:20.337568	2026-07-24 15:41:20.337568	remove authors test	\N
1037	Remove Genres Test	eng	0	novel		0	2026-07-24 15:41:20.354362	2026-07-24 15:41:20.354362	remove genres test	\N
1038	Remove Tags Test	eng	0	novel		0	2026-07-24 15:41:20.371216	2026-07-24 15:41:20.371216	remove tags test	\N
1039	Nil Authors Updated Title	eng	0	novel		0	2026-07-24 15:41:20.387168	2026-07-24 15:41:20.394458	nil authors updated title	\N
1040	Add Author Test	eng	0	novel		0	2026-07-24 15:41:20.404612	2026-07-24 15:41:20.404612	add author test	\N
1041	Test Book Part 1	eng	2023	novel	First test book created via API	0	2026-07-24 15:46:46.824363	2026-07-24 15:46:46.824363	test book part 1	\N
1042	Test Book Part 2	eng	2024	novel	Second test book created via API	0	2026-07-24 15:46:46.830658	2026-07-24 15:46:46.830658	test book part 2	\N
1043	Updated Book Title	eng	2022	novel	Updated description	0	2026-07-24 15:46:46.842188	2026-07-24 15:46:46.848502	updated book title	\N
1045	Updated Title	eng	0	novel	New annotation text	0	2026-07-24 15:46:46.878097	2026-07-24 15:46:46.88783	updated title	\N
1046	Updated Title Only	eng	0	novel		0	2026-07-24 15:46:46.899645	2026-07-24 15:46:46.905757	updated title only	\N
1047	Updated Title Empty ISBN	eng	0	novel		0	2026-07-24 15:46:46.91668	2026-07-24 15:46:46.922678	updated title empty isbn	\N
1048	Book One	eng	0	novel		0	2026-07-24 15:46:46.931845	2026-07-24 15:46:46.931845	book one	\N
1049	Book Two	eng	0	novel		0	2026-07-24 15:46:46.93769	2026-07-24 15:46:46.93769	book two	\N
1050	Original Book Title	eng	0	novel		0	2026-07-24 15:46:46.951962	2026-07-24 15:46:46.951962	original book title	\N
1051	Updated Title	eng	0	novel		0	2026-07-24 15:46:46.969325	2026-07-24 15:46:46.976173	updated title	\N
1052	Corrupted ISBN Test	eng	0	novel		0	2026-07-24 15:46:46.986683	2026-07-24 15:46:46.986683	corrupted isbn test	\N
1053	Remove Authors Test	eng	0	novel		0	2026-07-24 15:46:47.452354	2026-07-24 15:46:47.452354	remove authors test	\N
1054	Remove Genres Test	eng	0	novel		0	2026-07-24 15:46:47.470691	2026-07-24 15:46:47.470691	remove genres test	\N
1055	Remove Tags Test	eng	0	novel		0	2026-07-24 15:46:47.490046	2026-07-24 15:46:47.490046	remove tags test	\N
1056	Nil Authors Updated Title	eng	0	novel		0	2026-07-24 15:46:47.509323	2026-07-24 15:46:47.516865	nil authors updated title	\N
1057	Add Author Test	eng	0	novel		0	2026-07-24 15:46:47.527781	2026-07-24 15:46:47.527781	add author test	\N
1058	Test Book Part 1	eng	2023	novel	First test book created via API	0	2026-07-24 15:53:08.595811	2026-07-24 15:53:08.595811	test book part 1	\N
1059	Test Book Part 2	eng	2024	novel	Second test book created via API	0	2026-07-24 15:53:08.602755	2026-07-24 15:53:08.602755	test book part 2	\N
1060	Updated Book Title	eng	2022	novel	Updated description	0	2026-07-24 15:53:08.615847	2026-07-24 15:53:08.621967	updated book title	\N
1062	Updated Title	eng	0	novel	New annotation text	0	2026-07-24 15:53:08.650741	2026-07-24 15:53:08.661532	updated title	\N
1063	Updated Title Only	eng	0	novel		0	2026-07-24 15:53:08.673799	2026-07-24 15:53:08.680434	updated title only	\N
1064	Updated Title Empty ISBN	eng	0	novel		0	2026-07-24 15:53:08.692076	2026-07-24 15:53:08.697961	updated title empty isbn	\N
1065	Book One	eng	0	novel		0	2026-07-24 15:53:08.709321	2026-07-24 15:53:08.709321	book one	\N
1066	Book Two	eng	0	novel		0	2026-07-24 15:53:08.715887	2026-07-24 15:53:08.715887	book two	\N
1067	Original Book Title	eng	0	novel		0	2026-07-24 15:53:08.727616	2026-07-24 15:53:08.727616	original book title	\N
1068	Updated Title	eng	0	novel		0	2026-07-24 15:53:08.746351	2026-07-24 15:53:08.752835	updated title	\N
1069	Corrupted ISBN Test	eng	0	novel		0	2026-07-24 15:53:08.762448	2026-07-24 15:53:08.762448	corrupted isbn test	\N
1070	Remove Authors Test	eng	0	novel		0	2026-07-24 15:53:09.246651	2026-07-24 15:53:09.246651	remove authors test	\N
1071	Remove Genres Test	eng	0	novel		0	2026-07-24 15:53:09.264586	2026-07-24 15:53:09.264586	remove genres test	\N
1072	Remove Tags Test	eng	0	novel		0	2026-07-24 15:53:09.281413	2026-07-24 15:53:09.281413	remove tags test	\N
1073	Nil Authors Updated Title	eng	0	novel		0	2026-07-24 15:53:09.298026	2026-07-24 15:53:09.304365	nil authors updated title	\N
1074	Add Author Test	eng	0	novel		0	2026-07-24 15:53:09.31492	2026-07-24 15:53:09.31492	add author test	\N
1075	Test Book Part 1	eng	2023	novel	First test book created via API	0	2026-07-24 15:54:21.473135	2026-07-24 15:54:21.473135	test book part 1	\N
1076	Test Book Part 2	eng	2024	novel	Second test book created via API	0	2026-07-24 15:54:21.479475	2026-07-24 15:54:21.479475	test book part 2	\N
1077	Updated Book Title	eng	2022	novel	Updated description	0	2026-07-24 15:54:21.492925	2026-07-24 15:54:21.499231	updated book title	\N
1079	Updated Title	eng	0	novel	New annotation text	0	2026-07-24 15:54:21.532338	2026-07-24 15:54:21.543653	updated title	\N
1080	Updated Title Only	eng	0	novel		0	2026-07-24 15:54:21.555972	2026-07-24 15:54:21.562725	updated title only	\N
1081	Updated Title Empty ISBN	eng	0	novel		0	2026-07-24 15:54:21.573318	2026-07-24 15:54:21.581625	updated title empty isbn	\N
1082	Book One	eng	0	novel		0	2026-07-24 15:54:21.591606	2026-07-24 15:54:21.591606	book one	\N
1083	Book Two	eng	0	novel		0	2026-07-24 15:54:21.597821	2026-07-24 15:54:21.597821	book two	\N
1084	Original Book Title	eng	0	novel		0	2026-07-24 15:54:21.61436	2026-07-24 15:54:21.61436	original book title	\N
1085	Updated Title	eng	0	novel		0	2026-07-24 15:54:21.636086	2026-07-24 15:54:21.64422	updated title	\N
1086	Corrupted ISBN Test	eng	0	novel		0	2026-07-24 15:54:21.656271	2026-07-24 15:54:21.656271	corrupted isbn test	\N
1087	Remove Authors Test	eng	0	novel		0	2026-07-24 15:54:22.157474	2026-07-24 15:54:22.157474	remove authors test	\N
1088	Remove Genres Test	eng	0	novel		0	2026-07-24 15:54:22.177833	2026-07-24 15:54:22.177833	remove genres test	\N
1089	Remove Tags Test	eng	0	novel		0	2026-07-24 15:54:22.195172	2026-07-24 15:54:22.195172	remove tags test	\N
1090	Nil Authors Updated Title	eng	0	novel		0	2026-07-24 15:54:22.212479	2026-07-24 15:54:22.219231	nil authors updated title	\N
1091	Add Author Test	eng	0	novel		0	2026-07-24 15:54:22.228632	2026-07-24 15:54:22.228632	add author test	\N
1092	Test Book Part 1	eng	2023	novel	First test book created via API	0	2026-07-24 19:06:25.640595	2026-07-24 19:06:25.640595	test book part 1	\N
1093	Test Book Part 2	eng	2024	novel	Second test book created via API	0	2026-07-24 19:06:25.647304	2026-07-24 19:06:25.647304	test book part 2	\N
1094	Updated Book Title	eng	2022	novel	Updated description	0	2026-07-24 19:06:25.660861	2026-07-24 19:06:25.66685	updated book title	\N
1096	Updated Title	eng	0	novel	New annotation text	0	2026-07-24 19:06:25.700338	2026-07-24 19:06:25.710557	updated title	\N
1097	Updated Title Only	eng	0	novel		0	2026-07-24 19:06:25.723716	2026-07-24 19:06:25.729765	updated title only	\N
1098	Updated Title Empty ISBN	eng	0	novel		0	2026-07-24 19:06:25.746073	2026-07-24 19:06:25.753276	updated title empty isbn	\N
1099	Book One	eng	0	novel		0	2026-07-24 19:06:25.767342	2026-07-24 19:06:25.767342	book one	\N
1100	Book Two	eng	0	novel		0	2026-07-24 19:06:25.773304	2026-07-24 19:06:25.773304	book two	\N
1101	Original Book Title	eng	0	novel		0	2026-07-24 19:06:25.788612	2026-07-24 19:06:25.788612	original book title	\N
1102	Updated Title	eng	0	novel		0	2026-07-24 19:06:25.808211	2026-07-24 19:06:25.814443	updated title	\N
1103	Corrupted ISBN Test	eng	0	novel		0	2026-07-24 19:06:25.827394	2026-07-24 19:06:25.827394	corrupted isbn test	\N
1104	Remove Authors Test	eng	0	novel		0	2026-07-24 19:06:26.375313	2026-07-24 19:06:26.375313	remove authors test	\N
1105	Remove Genres Test	eng	0	novel		0	2026-07-24 19:06:26.393159	2026-07-24 19:06:26.393159	remove genres test	\N
1106	Remove Tags Test	eng	0	novel		0	2026-07-24 19:06:26.413001	2026-07-24 19:06:26.413001	remove tags test	\N
1107	Nil Authors Updated Title	eng	0	novel		0	2026-07-24 19:06:26.430468	2026-07-24 19:06:26.436935	nil authors updated title	\N
1108	Add Author Test	eng	0	novel		0	2026-07-24 19:06:26.446678	2026-07-24 19:06:26.446678	add author test	\N
1109	Test Book Part 1	eng	2023	novel	First test book created via API	0	2026-07-24 19:13:00.870128	2026-07-24 19:13:00.870128	test book part 1	\N
1110	Test Book Part 2	eng	2024	novel	Second test book created via API	0	2026-07-24 19:13:00.877423	2026-07-24 19:13:00.877423	test book part 2	\N
1111	Updated Book Title	eng	2022	novel	Updated description	0	2026-07-24 19:13:00.89039	2026-07-24 19:13:00.897218	updated book title	\N
1113	Updated Title	eng	0	novel	New annotation text	0	2026-07-24 19:13:00.927296	2026-07-24 19:13:00.937931	updated title	\N
1114	Updated Title Only	eng	0	novel		0	2026-07-24 19:13:00.950338	2026-07-24 19:13:00.957242	updated title only	\N
1115	Updated Title Empty ISBN	eng	0	novel		0	2026-07-24 19:13:00.974985	2026-07-24 19:13:00.980956	updated title empty isbn	\N
1116	Book One	eng	0	novel		0	2026-07-24 19:13:00.990697	2026-07-24 19:13:00.990697	book one	\N
1117	Book Two	eng	0	novel		0	2026-07-24 19:13:00.996987	2026-07-24 19:13:00.996987	book two	\N
1118	Original Book Title	eng	0	novel		0	2026-07-24 19:13:01.009138	2026-07-24 19:13:01.009138	original book title	\N
1119	Updated Title	eng	0	novel		0	2026-07-24 19:13:01.026899	2026-07-24 19:13:01.033244	updated title	\N
1120	Corrupted ISBN Test	eng	0	novel		0	2026-07-24 19:13:01.045419	2026-07-24 19:13:01.045419	corrupted isbn test	\N
1121	Remove Authors Test	eng	0	novel		0	2026-07-24 19:13:01.568701	2026-07-24 19:13:01.568701	remove authors test	\N
1122	Remove Genres Test	eng	0	novel		0	2026-07-24 19:13:01.586883	2026-07-24 19:13:01.586883	remove genres test	\N
1123	Remove Tags Test	eng	0	novel		0	2026-07-24 19:13:01.609914	2026-07-24 19:13:01.609914	remove tags test	\N
1124	Nil Authors Updated Title	eng	0	novel		0	2026-07-24 19:13:01.628404	2026-07-24 19:13:01.634327	nil authors updated title	\N
1125	Add Author Test	eng	0	novel		0	2026-07-24 19:13:01.649206	2026-07-24 19:13:01.649206	add author test	\N
1126	Test Book Part 1	eng	2023	novel	First test book created via API	0	2026-07-24 19:47:46.415981	2026-07-24 19:47:46.415981	test book part 1	\N
1127	Test Book Part 2	eng	2024	novel	Second test book created via API	0	2026-07-24 19:47:46.422245	2026-07-24 19:47:46.422245	test book part 2	\N
1128	Updated Book Title	eng	2022	novel	Updated description	0	2026-07-24 19:47:46.433338	2026-07-24 19:47:46.439252	updated book title	\N
1130	Updated Title	eng	0	novel	New annotation text	0	2026-07-24 19:47:46.471255	2026-07-24 19:47:46.48211	updated title	\N
1131	Updated Title Only	eng	0	novel		0	2026-07-24 19:47:46.493131	2026-07-24 19:47:46.499222	updated title only	\N
1132	Updated Title Empty ISBN	eng	0	novel		0	2026-07-24 19:47:46.510162	2026-07-24 19:47:46.513931	updated title empty isbn	\N
1133	Book One	eng	0	novel		0	2026-07-24 19:47:46.525577	2026-07-24 19:47:46.525577	book one	\N
1134	Book Two	eng	0	novel		0	2026-07-24 19:47:46.531306	2026-07-24 19:47:46.531306	book two	\N
1135	Original Book Title	eng	0	novel		0	2026-07-24 19:47:46.544987	2026-07-24 19:47:46.544987	original book title	\N
1136	Updated Title	eng	0	novel		0	2026-07-24 19:47:46.566817	2026-07-24 19:47:46.573652	updated title	\N
1137	Corrupted ISBN Test	eng	0	novel		0	2026-07-24 19:47:46.582561	2026-07-24 19:47:46.582561	corrupted isbn test	\N
1138	Remove Authors Test	eng	0	novel		0	2026-07-24 19:47:47.086925	2026-07-24 19:47:47.086925	remove authors test	\N
1139	Remove Genres Test	eng	0	novel		0	2026-07-24 19:47:47.105484	2026-07-24 19:47:47.105484	remove genres test	\N
1140	Remove Tags Test	eng	0	novel		0	2026-07-24 19:47:47.131975	2026-07-24 19:47:47.131975	remove tags test	\N
1141	Nil Authors Updated Title	eng	0	novel		0	2026-07-24 19:47:47.14891	2026-07-24 19:47:47.154952	nil authors updated title	\N
1142	Add Author Test	eng	0	novel		0	2026-07-24 19:47:47.163993	2026-07-24 19:47:47.163993	add author test	\N
1143	Test Book Part 1	eng	2023	novel	First test book created via API	0	2026-07-24 19:48:03.634369	2026-07-24 19:48:03.634369	test book part 1	\N
1144	Test Book Part 2	eng	2024	novel	Second test book created via API	0	2026-07-24 19:48:03.641471	2026-07-24 19:48:03.641471	test book part 2	\N
1145	Updated Book Title	eng	2022	novel	Updated description	0	2026-07-24 19:48:03.654486	2026-07-24 19:48:03.660949	updated book title	\N
1147	Updated Title	eng	0	novel	New annotation text	0	2026-07-24 19:48:03.691668	2026-07-24 19:48:03.703168	updated title	\N
1148	Updated Title Only	eng	0	novel		0	2026-07-24 19:48:03.715392	2026-07-24 19:48:03.721525	updated title only	\N
1149	Updated Title Empty ISBN	eng	0	novel		0	2026-07-24 19:48:03.735367	2026-07-24 19:48:03.74157	updated title empty isbn	\N
1150	Book One	eng	0	novel		0	2026-07-24 19:48:03.752975	2026-07-24 19:48:03.752975	book one	\N
1151	Book Two	eng	0	novel		0	2026-07-24 19:48:03.759338	2026-07-24 19:48:03.759338	book two	\N
1152	Original Book Title	eng	0	novel		0	2026-07-24 19:48:03.771414	2026-07-24 19:48:03.771414	original book title	\N
1153	Updated Title	eng	0	novel		0	2026-07-24 19:48:03.790649	2026-07-24 19:48:03.799996	updated title	\N
1154	Corrupted ISBN Test	eng	0	novel		0	2026-07-24 19:48:03.819755	2026-07-24 19:48:03.819755	corrupted isbn test	\N
1155	Remove Authors Test	eng	0	novel		0	2026-07-24 19:48:04.339376	2026-07-24 19:48:04.339376	remove authors test	\N
1156	Remove Genres Test	eng	0	novel		0	2026-07-24 19:48:04.356463	2026-07-24 19:48:04.356463	remove genres test	\N
1157	Remove Tags Test	eng	0	novel		0	2026-07-24 19:48:04.379497	2026-07-24 19:48:04.379497	remove tags test	\N
1158	Nil Authors Updated Title	eng	0	novel		0	2026-07-24 19:48:04.39956	2026-07-24 19:48:04.406041	nil authors updated title	\N
1159	Add Author Test	eng	0	novel		0	2026-07-24 19:48:04.418486	2026-07-24 19:48:04.418486	add author test	\N
1160	Test Book Part 1	eng	2023	novel	First test book created via API	0	2026-07-25 11:35:26.597435	2026-07-25 11:35:26.597435	test book part 1	\N
1161	Test Book Part 2	eng	2024	novel	Second test book created via API	0	2026-07-25 11:35:26.618086	2026-07-25 11:35:26.618086	test book part 2	\N
1162	Updated Book Title	eng	2022	novel	Updated description	0	2026-07-25 11:35:26.629088	2026-07-25 11:35:26.636269	updated book title	\N
1164	Updated Title	eng	0	novel	New annotation text	0	2026-07-25 11:35:26.670686	2026-07-25 11:35:26.681978	updated title	\N
1165	Updated Title Only	eng	0	novel		0	2026-07-25 11:35:26.695544	2026-07-25 11:35:26.701603	updated title only	\N
1166	Updated Title Empty ISBN	eng	0	novel		0	2026-07-25 11:35:26.71329	2026-07-25 11:35:26.719518	updated title empty isbn	\N
1167	Book One	eng	0	novel		0	2026-07-25 11:35:26.729228	2026-07-25 11:35:26.729228	book one	\N
1168	Book Two	eng	0	novel		0	2026-07-25 11:35:26.737323	2026-07-25 11:35:26.737323	book two	\N
1169	Original Book Title	eng	0	novel		0	2026-07-25 11:35:26.7503	2026-07-25 11:35:26.7503	original book title	\N
1170	Updated Title	eng	0	novel		0	2026-07-25 11:35:26.767563	2026-07-25 11:35:26.773938	updated title	\N
1171	Corrupted ISBN Test	eng	0	novel		0	2026-07-25 11:35:26.785185	2026-07-25 11:35:26.785185	corrupted isbn test	\N
1172	Remove Authors Test	eng	0	novel		0	2026-07-25 11:35:27.277602	2026-07-25 11:35:27.277602	remove authors test	\N
1173	Remove Genres Test	eng	0	novel		0	2026-07-25 11:35:27.295264	2026-07-25 11:35:27.295264	remove genres test	\N
1174	Remove Tags Test	eng	0	novel		0	2026-07-25 11:35:27.312914	2026-07-25 11:35:27.312914	remove tags test	\N
1175	Nil Authors Updated Title	eng	0	novel		0	2026-07-25 11:35:27.328963	2026-07-25 11:35:27.33603	nil authors updated title	\N
1176	Add Author Test	eng	0	novel		0	2026-07-25 11:35:27.345399	2026-07-25 11:35:27.345399	add author test	\N
1177	Test Book Part 1	eng	2023	novel	First test book created via API	0	2026-07-25 14:19:39.849752	2026-07-25 14:19:39.849752	test book part 1	\N
1178	Test Book Part 2	eng	2024	novel	Second test book created via API	0	2026-07-25 14:19:39.858317	2026-07-25 14:19:39.858317	test book part 2	\N
1179	Updated Book Title	eng	2022	novel	Updated description	0	2026-07-25 14:19:39.871557	2026-07-25 14:19:39.877374	updated book title	\N
1181	Updated Title	eng	0	novel	New annotation text	0	2026-07-25 14:19:39.911022	2026-07-25 14:19:39.921561	updated title	\N
1182	Updated Title Only	eng	0	novel		0	2026-07-25 14:19:39.940497	2026-07-25 14:19:39.947177	updated title only	\N
1183	Updated Title Empty ISBN	eng	0	novel		0	2026-07-25 14:19:39.957652	2026-07-25 14:19:39.964833	updated title empty isbn	\N
1184	Book One	eng	0	novel		0	2026-07-25 14:19:39.974833	2026-07-25 14:19:39.974833	book one	\N
1185	Book Two	eng	0	novel		0	2026-07-25 14:19:39.980905	2026-07-25 14:19:39.980905	book two	\N
1186	Original Book Title	eng	0	novel		0	2026-07-25 14:19:39.994795	2026-07-25 14:19:39.994795	original book title	\N
1187	Updated Title	eng	0	novel		0	2026-07-25 14:19:40.014643	2026-07-25 14:19:40.02106	updated title	\N
1188	Corrupted ISBN Test	eng	0	novel		0	2026-07-25 14:19:40.03149	2026-07-25 14:19:40.03149	corrupted isbn test	\N
1189	Remove Authors Test	eng	0	novel		0	2026-07-25 14:19:40.517551	2026-07-25 14:19:40.517551	remove authors test	\N
1190	Remove Genres Test	eng	0	novel		0	2026-07-25 14:19:40.536549	2026-07-25 14:19:40.536549	remove genres test	\N
1191	Remove Tags Test	eng	0	novel		0	2026-07-25 14:19:40.556586	2026-07-25 14:19:40.556586	remove tags test	\N
1192	Nil Authors Updated Title	eng	0	novel		0	2026-07-25 14:19:40.574113	2026-07-25 14:19:40.580131	nil authors updated title	\N
1193	Add Author Test	eng	0	novel		0	2026-07-25 14:19:40.59136	2026-07-25 14:19:40.59136	add author test	\N
1194	Test Book Part 1	eng	2023	novel	First test book created via API	0	2026-07-25 15:04:15.330683	2026-07-25 15:04:15.330683	test book part 1	\N
1195	Test Book Part 2	eng	2024	novel	Second test book created via API	0	2026-07-25 15:04:15.33773	2026-07-25 15:04:15.33773	test book part 2	\N
1196	Updated Book Title	eng	2022	novel	Updated description	0	2026-07-25 15:04:15.349899	2026-07-25 15:04:15.356658	updated book title	\N
1198	Updated Title	eng	0	novel	New annotation text	0	2026-07-25 15:04:15.388347	2026-07-25 15:04:15.398317	updated title	\N
1199	Updated Title Only	eng	0	novel		0	2026-07-25 15:04:15.410197	2026-07-25 15:04:15.417194	updated title only	\N
1200	Updated Title Empty ISBN	eng	0	novel		0	2026-07-25 15:04:15.429545	2026-07-25 15:04:15.435917	updated title empty isbn	\N
1201	Book One	eng	0	novel		0	2026-07-25 15:04:15.449545	2026-07-25 15:04:15.449545	book one	\N
1202	Book Two	eng	0	novel		0	2026-07-25 15:04:15.455178	2026-07-25 15:04:15.455178	book two	\N
1203	Original Book Title	eng	0	novel		0	2026-07-25 15:04:15.467931	2026-07-25 15:04:15.467931	original book title	\N
1204	Updated Title	eng	0	novel		0	2026-07-25 15:04:15.485409	2026-07-25 15:04:15.491807	updated title	\N
1205	Corrupted ISBN Test	eng	0	novel		0	2026-07-25 15:04:15.501876	2026-07-25 15:04:15.501876	corrupted isbn test	\N
1206	Remove Authors Test	eng	0	novel		0	2026-07-25 15:04:16.008767	2026-07-25 15:04:16.008767	remove authors test	\N
1207	Remove Genres Test	eng	0	novel		0	2026-07-25 15:04:16.027255	2026-07-25 15:04:16.027255	remove genres test	\N
1208	Remove Tags Test	eng	0	novel		0	2026-07-25 15:04:16.044203	2026-07-25 15:04:16.044203	remove tags test	\N
1209	Nil Authors Updated Title	eng	0	novel		0	2026-07-25 15:04:16.060537	2026-07-25 15:04:16.067663	nil authors updated title	\N
1210	Add Author Test	eng	0	novel		0	2026-07-25 15:04:16.078449	2026-07-25 15:04:16.078449	add author test	\N
1211	Test Book Part 1	eng	2023	novel	First test book created via API	0	2026-07-25 15:29:01.853788	2026-07-25 15:29:01.853788	test book part 1	\N
1212	Test Book Part 2	eng	2024	novel	Second test book created via API	0	2026-07-25 15:29:01.86069	2026-07-25 15:29:01.86069	test book part 2	\N
1213	Updated Book Title	eng	2022	novel	Updated description	0	2026-07-25 15:29:01.872967	2026-07-25 15:29:01.879385	updated book title	\N
1215	Updated Title	eng	0	novel	New annotation text	0	2026-07-25 15:29:01.909197	2026-07-25 15:29:01.920728	updated title	\N
1216	Updated Title Only	eng	0	novel		0	2026-07-25 15:29:01.932952	2026-07-25 15:29:01.938982	updated title only	\N
1217	Updated Title Empty ISBN	eng	0	novel		0	2026-07-25 15:29:01.949971	2026-07-25 15:29:01.955888	updated title empty isbn	\N
1218	Book One	eng	0	novel		0	2026-07-25 15:29:01.975503	2026-07-25 15:29:01.975503	book one	\N
1219	Book Two	eng	0	novel		0	2026-07-25 15:29:01.982144	2026-07-25 15:29:01.982144	book two	\N
1220	Original Book Title	eng	0	novel		0	2026-07-25 15:29:01.994111	2026-07-25 15:29:01.994111	original book title	\N
1221	Updated Title	eng	0	novel		0	2026-07-25 15:29:02.012812	2026-07-25 15:29:02.01884	updated title	\N
1222	Corrupted ISBN Test	eng	0	novel		0	2026-07-25 15:29:02.029	2026-07-25 15:29:02.029	corrupted isbn test	\N
1223	Remove Authors Test	eng	0	novel		0	2026-07-25 15:29:02.519947	2026-07-25 15:29:02.519947	remove authors test	\N
1224	Remove Genres Test	eng	0	novel		0	2026-07-25 15:29:02.538807	2026-07-25 15:29:02.538807	remove genres test	\N
1225	Remove Tags Test	eng	0	novel		0	2026-07-25 15:29:02.557888	2026-07-25 15:29:02.557888	remove tags test	\N
1226	Nil Authors Updated Title	eng	0	novel		0	2026-07-25 15:29:02.574292	2026-07-25 15:29:02.580414	nil authors updated title	\N
1227	Add Author Test	eng	0	novel		0	2026-07-25 15:29:02.589087	2026-07-25 15:29:02.589087	add author test	\N
1228	Test Book Part 1	eng	2023	novel	First test book created via API	0	2026-07-25 15:47:11.629139	2026-07-25 15:47:11.629139	test book part 1	\N
1229	Test Book Part 2	eng	2024	novel	Second test book created via API	0	2026-07-25 15:47:11.635446	2026-07-25 15:47:11.635446	test book part 2	\N
1230	Updated Book Title	eng	2022	novel	Updated description	0	2026-07-25 15:47:11.65204	2026-07-25 15:47:11.658144	updated book title	\N
1232	Updated Title	eng	0	novel	New annotation text	0	2026-07-25 15:47:11.692465	2026-07-25 15:47:11.703343	updated title	\N
1233	Updated Title Only	eng	0	novel		0	2026-07-25 15:47:11.716246	2026-07-25 15:47:11.722147	updated title only	\N
1234	Updated Title Empty ISBN	eng	0	novel		0	2026-07-25 15:47:11.735915	2026-07-25 15:47:11.742695	updated title empty isbn	\N
1235	Book One	eng	0	novel		0	2026-07-25 15:47:11.754176	2026-07-25 15:47:11.754176	book one	\N
1236	Book Two	eng	0	novel		0	2026-07-25 15:47:11.760246	2026-07-25 15:47:11.760246	book two	\N
1237	Original Book Title	eng	0	novel		0	2026-07-25 15:47:11.781467	2026-07-25 15:47:11.781467	original book title	\N
1238	Updated Title	eng	0	novel		0	2026-07-25 15:47:11.801097	2026-07-25 15:47:11.807737	updated title	\N
1239	Corrupted ISBN Test	eng	0	novel		0	2026-07-25 15:47:11.820388	2026-07-25 15:47:11.820388	corrupted isbn test	\N
1240	Remove Authors Test	eng	0	novel		0	2026-07-25 15:47:12.333721	2026-07-25 15:47:12.333721	remove authors test	\N
1241	Remove Genres Test	eng	0	novel		0	2026-07-25 15:47:12.360108	2026-07-25 15:47:12.360108	remove genres test	\N
1242	Remove Tags Test	eng	0	novel		0	2026-07-25 15:47:12.393971	2026-07-25 15:47:12.393971	remove tags test	\N
1243	Nil Authors Updated Title	eng	0	novel		0	2026-07-25 15:47:12.414296	2026-07-25 15:47:12.420882	nil authors updated title	\N
1244	Add Author Test	eng	0	novel		0	2026-07-25 15:47:12.442687	2026-07-25 15:47:12.442687	add author test	\N
795	Иуда Искариот	rus	1907	novel	«Иисуса Христа много раз предупреждали, что Иуда из Кариота – человек очень дурной славы и его нужно остерегаться. Одни из учеников, бывавшие в Иудее, хорошо знали его сами, другие много слыхали о нем от людей, и не было никого, кто мог бы сказать о нем доброе слово. И если порицали его добрые, говоря, что Иуда корыстолюбив, коварен, наклонен к притворству и лжи, то и дурные, которых расспрашивали об Иуде, поносили его самыми жестокими словами. «Он ссорит нас постоянно, – говорили они, отплевываясь, – он думает что-то свое и в дом влезает тихо, как скорпион, а выходит из него с шумом…»	\N	2026-07-21 15:46:29.382077	2026-07-25 15:52:53.676455	иуда искариот	\N
1245	Test Book Part 1	eng	2023	novel	First test book created via API	0	2026-07-25 15:54:34.743911	2026-07-25 15:54:34.743911	test book part 1	\N
1246	Test Book Part 2	eng	2024	novel	Second test book created via API	0	2026-07-25 15:54:34.749968	2026-07-25 15:54:34.749968	test book part 2	\N
1247	Updated Book Title	eng	2022	novel	Updated description	0	2026-07-25 15:54:34.760406	2026-07-25 15:54:34.766309	updated book title	\N
1249	Updated Title	eng	0	novel	New annotation text	0	2026-07-25 15:54:34.793323	2026-07-25 15:54:34.803815	updated title	\N
1250	Updated Title Only	eng	0	novel		0	2026-07-25 15:54:34.816539	2026-07-25 15:54:34.822982	updated title only	\N
1251	Updated Title Empty ISBN	eng	0	novel		0	2026-07-25 15:54:34.834187	2026-07-25 15:54:34.839903	updated title empty isbn	\N
1252	Book One	eng	0	novel		0	2026-07-25 15:54:34.848658	2026-07-25 15:54:34.848658	book one	\N
1253	Book Two	eng	0	novel		0	2026-07-25 15:54:34.854424	2026-07-25 15:54:34.854424	book two	\N
1254	Original Book Title	eng	0	novel		0	2026-07-25 15:54:34.867163	2026-07-25 15:54:34.867163	original book title	\N
1255	Updated Title	eng	0	novel		0	2026-07-25 15:54:34.887096	2026-07-25 15:54:34.893806	updated title	\N
1256	Corrupted ISBN Test	eng	0	novel		0	2026-07-25 15:54:34.904952	2026-07-25 15:54:34.904952	corrupted isbn test	\N
1257	Remove Authors Test	eng	0	novel		0	2026-07-25 15:54:35.392974	2026-07-25 15:54:35.392974	remove authors test	\N
1258	Remove Genres Test	eng	0	novel		0	2026-07-25 15:54:35.411116	2026-07-25 15:54:35.411116	remove genres test	\N
1259	Remove Tags Test	eng	0	novel		0	2026-07-25 15:54:35.42709	2026-07-25 15:54:35.42709	remove tags test	\N
1260	Nil Authors Updated Title	eng	0	novel		0	2026-07-25 15:54:35.444918	2026-07-25 15:54:35.451248	nil authors updated title	\N
1261	Add Author Test	eng	0	novel		0	2026-07-25 15:54:35.463791	2026-07-25 15:54:35.463791	add author test	\N
1262	Test Book Part 1	eng	2023	novel	First test book created via API	0	2026-07-26 19:34:10.629768	2026-07-26 19:34:10.629768	test book part 1	\N
1263	Test Book Part 2	eng	2024	novel	Second test book created via API	0	2026-07-26 19:34:10.64703	2026-07-26 19:34:10.64703	test book part 2	\N
1264	Updated Book Title	eng	2022	novel	Updated description	0	2026-07-26 19:34:10.665086	2026-07-26 19:34:10.672009	updated book title	\N
1266	Updated Title	eng	0	novel	New annotation text	0	2026-07-26 19:34:10.711772	2026-07-26 19:34:10.724922	updated title	\N
1267	Updated Title Only	eng	0	novel		0	2026-07-26 19:34:10.738556	2026-07-26 19:34:10.745306	updated title only	\N
1268	Updated Title Empty ISBN	eng	0	novel		0	2026-07-26 19:34:10.760565	2026-07-26 19:34:10.766706	updated title empty isbn	\N
1269	Book One	eng	0	novel		0	2026-07-26 19:34:10.778269	2026-07-26 19:34:10.778269	book one	\N
1270	Book Two	eng	0	novel		0	2026-07-26 19:34:10.784682	2026-07-26 19:34:10.784682	book two	\N
1271	Original Book Title	eng	0	novel		0	2026-07-26 19:34:10.799533	2026-07-26 19:34:10.799533	original book title	\N
1272	Updated Title	eng	0	novel		0	2026-07-26 19:34:10.818841	2026-07-26 19:34:10.826231	updated title	\N
1273	Corrupted ISBN Test	eng	0	novel		0	2026-07-26 19:34:10.84004	2026-07-26 19:34:10.84004	corrupted isbn test	\N
1274	Remove Authors Test	eng	0	novel		0	2026-07-26 19:34:11.40391	2026-07-26 19:34:11.40391	remove authors test	\N
1275	Remove Genres Test	eng	0	novel		0	2026-07-26 19:34:11.42334	2026-07-26 19:34:11.42334	remove genres test	\N
1276	Remove Tags Test	eng	0	novel		0	2026-07-26 19:34:11.44492	2026-07-26 19:34:11.44492	remove tags test	\N
1277	Nil Authors Updated Title	eng	0	novel		0	2026-07-26 19:34:11.46355	2026-07-26 19:34:11.470241	nil authors updated title	\N
1278	Add Author Test	eng	0	novel		0	2026-07-26 19:34:11.480723	2026-07-26 19:34:11.480723	add author test	\N
1279	Test Book Part 1	eng	2023	novel	First test book created via API	0	2026-07-26 20:03:20.490721	2026-07-26 20:03:20.490721	test book part 1	\N
1280	Test Book Part 2	eng	2024	novel	Second test book created via API	0	2026-07-26 20:03:20.497074	2026-07-26 20:03:20.497074	test book part 2	\N
1281	Updated Book Title	eng	2022	novel	Updated description	0	2026-07-26 20:03:20.508698	2026-07-26 20:03:20.514734	updated book title	\N
1283	Updated Title	eng	0	novel	New annotation text	0	2026-07-26 20:03:20.545579	2026-07-26 20:03:20.557175	updated title	\N
1284	Updated Title Only	eng	0	novel		0	2026-07-26 20:03:20.571758	2026-07-26 20:03:20.577516	updated title only	\N
1285	Updated Title Empty ISBN	eng	0	novel		0	2026-07-26 20:03:20.588725	2026-07-26 20:03:20.595139	updated title empty isbn	\N
1286	Book One	eng	0	novel		0	2026-07-26 20:03:20.604812	2026-07-26 20:03:20.604812	book one	\N
1287	Book Two	eng	0	novel		0	2026-07-26 20:03:20.611036	2026-07-26 20:03:20.611036	book two	\N
1288	Original Book Title	eng	0	novel		0	2026-07-26 20:03:20.623674	2026-07-26 20:03:20.623674	original book title	\N
1289	Updated Title	eng	0	novel		0	2026-07-26 20:03:20.640799	2026-07-26 20:03:20.647324	updated title	\N
1290	Corrupted ISBN Test	eng	0	novel		0	2026-07-26 20:03:20.656595	2026-07-26 20:03:20.656595	corrupted isbn test	\N
1291	Remove Authors Test	eng	0	novel		0	2026-07-26 20:03:21.218299	2026-07-26 20:03:21.218299	remove authors test	\N
1292	Remove Genres Test	eng	0	novel		0	2026-07-26 20:03:21.235127	2026-07-26 20:03:21.235127	remove genres test	\N
1293	Remove Tags Test	eng	0	novel		0	2026-07-26 20:03:21.255066	2026-07-26 20:03:21.255066	remove tags test	\N
1294	Nil Authors Updated Title	eng	0	novel		0	2026-07-26 20:03:21.271093	2026-07-26 20:03:21.278215	nil authors updated title	\N
1295	Add Author Test	eng	0	novel		0	2026-07-26 20:03:21.286791	2026-07-26 20:03:21.286791	add author test	\N
1296	Test Book Part 1	eng	2023	novel	First test book created via API	0	2026-07-26 20:50:17.488358	2026-07-26 20:50:17.488358	test book part 1	\N
1297	Test Book Part 2	eng	2024	novel	Second test book created via API	0	2026-07-26 20:50:17.497292	2026-07-26 20:50:17.497292	test book part 2	\N
1298	Updated Book Title	eng	2022	novel	Updated description	0	2026-07-26 20:50:17.50793	2026-07-26 20:50:17.514947	updated book title	\N
1300	Updated Title	eng	0	novel	New annotation text	0	2026-07-26 20:50:17.546186	2026-07-26 20:50:17.558534	updated title	\N
1301	Updated Title Only	eng	0	novel		0	2026-07-26 20:50:17.573667	2026-07-26 20:50:17.579969	updated title only	\N
1302	Updated Title Empty ISBN	eng	0	novel		0	2026-07-26 20:50:17.594869	2026-07-26 20:50:17.601048	updated title empty isbn	\N
1303	Book One	eng	0	novel		0	2026-07-26 20:50:17.611735	2026-07-26 20:50:17.611735	book one	\N
1304	Book Two	eng	0	novel		0	2026-07-26 20:50:17.617712	2026-07-26 20:50:17.617712	book two	\N
1305	Original Book Title	eng	0	novel		0	2026-07-26 20:50:17.630028	2026-07-26 20:50:17.630028	original book title	\N
1306	Updated Title	eng	0	novel		0	2026-07-26 20:50:17.647073	2026-07-26 20:50:17.653717	updated title	\N
1307	Corrupted ISBN Test	eng	0	novel		0	2026-07-26 20:50:17.666102	2026-07-26 20:50:17.666102	corrupted isbn test	\N
1308	Remove Authors Test	eng	0	novel		0	2026-07-26 20:50:18.183151	2026-07-26 20:50:18.183151	remove authors test	\N
1309	Remove Genres Test	eng	0	novel		0	2026-07-26 20:50:18.204483	2026-07-26 20:50:18.204483	remove genres test	\N
1310	Remove Tags Test	eng	0	novel		0	2026-07-26 20:50:18.223243	2026-07-26 20:50:18.223243	remove tags test	\N
1311	Nil Authors Updated Title	eng	0	novel		0	2026-07-26 20:50:18.239006	2026-07-26 20:50:18.245168	nil authors updated title	\N
1312	Add Author Test	eng	0	novel		0	2026-07-26 20:50:18.25431	2026-07-26 20:50:18.25431	add author test	\N
1313	Test Book Part 1	eng	2023	novel	First test book created via API	0	2026-07-26 21:03:21.375001	2026-07-26 21:03:21.375001	test book part 1	\N
1314	Test Book Part 2	eng	2024	novel	Second test book created via API	0	2026-07-26 21:03:21.381478	2026-07-26 21:03:21.381478	test book part 2	\N
1315	Updated Book Title	eng	2022	novel	Updated description	0	2026-07-26 21:03:21.393274	2026-07-26 21:03:21.399804	updated book title	\N
1317	Updated Title	eng	0	novel	New annotation text	0	2026-07-26 21:03:21.427703	2026-07-26 21:03:21.438094	updated title	\N
1318	Updated Title Only	eng	0	novel		0	2026-07-26 21:03:21.451382	2026-07-26 21:03:21.457409	updated title only	\N
1319	Updated Title Empty ISBN	eng	0	novel		0	2026-07-26 21:03:21.469133	2026-07-26 21:03:21.475407	updated title empty isbn	\N
1320	Book One	eng	0	novel		0	2026-07-26 21:03:21.486114	2026-07-26 21:03:21.486114	book one	\N
1321	Book Two	eng	0	novel		0	2026-07-26 21:03:21.491856	2026-07-26 21:03:21.491856	book two	\N
1322	Original Book Title	eng	0	novel		0	2026-07-26 21:03:21.504041	2026-07-26 21:03:21.504041	original book title	\N
1323	Updated Title	eng	0	novel		0	2026-07-26 21:03:21.52175	2026-07-26 21:03:21.528972	updated title	\N
1324	Corrupted ISBN Test	eng	0	novel		0	2026-07-26 21:03:21.538295	2026-07-26 21:03:21.538295	corrupted isbn test	\N
1325	Remove Authors Test	eng	0	novel		0	2026-07-26 21:03:22.038481	2026-07-26 21:03:22.038481	remove authors test	\N
1326	Remove Genres Test	eng	0	novel		0	2026-07-26 21:03:22.055907	2026-07-26 21:03:22.055907	remove genres test	\N
1327	Remove Tags Test	eng	0	novel		0	2026-07-26 21:03:22.072202	2026-07-26 21:03:22.072202	remove tags test	\N
1328	Nil Authors Updated Title	eng	0	novel		0	2026-07-26 21:03:22.091257	2026-07-26 21:03:22.09777	nil authors updated title	\N
1329	Add Author Test	eng	0	novel		0	2026-07-26 21:03:22.107418	2026-07-26 21:03:22.107418	add author test	\N
1330	Test Book Part 1	eng	2023	novel	First test book created via API	0	2026-07-27 07:08:39.906875	2026-07-27 07:08:39.906875	test book part 1	\N
1331	Test Book Part 2	eng	2024	novel	Second test book created via API	0	2026-07-27 07:08:39.913707	2026-07-27 07:08:39.913707	test book part 2	\N
1332	Updated Book Title	eng	2022	novel	Updated description	0	2026-07-27 07:08:39.924879	2026-07-27 07:08:39.930505	updated book title	\N
1334	Updated Title	eng	0	novel	New annotation text	0	2026-07-27 07:08:39.963112	2026-07-27 07:08:39.973921	updated title	\N
1335	Updated Title Only	eng	0	novel		0	2026-07-27 07:08:39.98599	2026-07-27 07:08:39.992453	updated title only	\N
1336	Updated Title Empty ISBN	eng	0	novel		0	2026-07-27 07:08:40.004416	2026-07-27 07:08:40.010495	updated title empty isbn	\N
1337	Book One	eng	0	novel		0	2026-07-27 07:08:40.020814	2026-07-27 07:08:40.020814	book one	\N
1338	Book Two	eng	0	novel		0	2026-07-27 07:08:40.027235	2026-07-27 07:08:40.027235	book two	\N
1339	Original Book Title	eng	0	novel		0	2026-07-27 07:08:40.041454	2026-07-27 07:08:40.041454	original book title	\N
1340	Updated Title	eng	0	novel		0	2026-07-27 07:08:40.061855	2026-07-27 07:08:40.068832	updated title	\N
1341	Corrupted ISBN Test	eng	0	novel		0	2026-07-27 07:08:40.079275	2026-07-27 07:08:40.079275	corrupted isbn test	\N
1342	Remove Authors Test	eng	0	novel		0	2026-07-27 07:08:41.210459	2026-07-27 07:08:41.210459	remove authors test	\N
1343	Remove Genres Test	eng	0	novel		0	2026-07-27 07:08:41.226276	2026-07-27 07:08:41.226276	remove genres test	\N
1344	Remove Tags Test	eng	0	novel		0	2026-07-27 07:08:41.242867	2026-07-27 07:08:41.242867	remove tags test	\N
1345	Nil Authors Updated Title	eng	0	novel		0	2026-07-27 07:08:41.25961	2026-07-27 07:08:41.265784	nil authors updated title	\N
1346	Add Author Test	eng	0	novel		0	2026-07-27 07:08:41.277766	2026-07-27 07:08:41.277766	add author test	\N
1347	Test Book Part 1	eng	2023	novel	First test book created via API	0	2026-07-27 07:14:51.216994	2026-07-27 07:14:51.216994	test book part 1	\N
1348	Test Book Part 2	eng	2024	novel	Second test book created via API	0	2026-07-27 07:14:51.224802	2026-07-27 07:14:51.224802	test book part 2	\N
1349	Updated Book Title	eng	2022	novel	Updated description	0	2026-07-27 07:14:51.240208	2026-07-27 07:14:51.246065	updated book title	\N
1351	Updated Title	eng	0	novel	New annotation text	0	2026-07-27 07:14:51.2788	2026-07-27 07:14:51.289876	updated title	\N
1352	Updated Title Only	eng	0	novel		0	2026-07-27 07:14:51.302791	2026-07-27 07:14:51.308938	updated title only	\N
1353	Updated Title Empty ISBN	eng	0	novel		0	2026-07-27 07:14:51.32056	2026-07-27 07:14:51.326399	updated title empty isbn	\N
1354	Book One	eng	0	novel		0	2026-07-27 07:14:51.336307	2026-07-27 07:14:51.336307	book one	\N
1355	Book Two	eng	0	novel		0	2026-07-27 07:14:51.342091	2026-07-27 07:14:51.342091	book two	\N
1356	Original Book Title	eng	0	novel		0	2026-07-27 07:14:51.360027	2026-07-27 07:14:51.360027	original book title	\N
1357	Updated Title	eng	0	novel		0	2026-07-27 07:14:51.377868	2026-07-27 07:14:51.385096	updated title	\N
1358	Corrupted ISBN Test	eng	0	novel		0	2026-07-27 07:14:51.394203	2026-07-27 07:14:51.394203	corrupted isbn test	\N
1359	Remove Authors Test	eng	0	novel		0	2026-07-27 07:14:52.509815	2026-07-27 07:14:52.509815	remove authors test	\N
1360	Remove Genres Test	eng	0	novel		0	2026-07-27 07:14:52.52767	2026-07-27 07:14:52.52767	remove genres test	\N
1361	Remove Tags Test	eng	0	novel		0	2026-07-27 07:14:52.544735	2026-07-27 07:14:52.544735	remove tags test	\N
1362	Nil Authors Updated Title	eng	0	novel		0	2026-07-27 07:14:52.56123	2026-07-27 07:14:52.567858	nil authors updated title	\N
1363	Add Author Test	eng	0	novel		0	2026-07-27 07:14:52.579477	2026-07-27 07:14:52.579477	add author test	\N
1364	Test Book Part 1	eng	2023	novel	First test book created via API	0	2026-07-27 10:12:33.25546	2026-07-27 10:12:33.25546	test book part 1	\N
1365	Test Book Part 2	eng	2024	novel	Second test book created via API	0	2026-07-27 10:12:33.263035	2026-07-27 10:12:33.263035	test book part 2	\N
1366	Updated Book Title	eng	2022	novel	Updated description	0	2026-07-27 10:12:33.274156	2026-07-27 10:12:33.279953	updated book title	\N
1368	Updated Title	eng	0	novel	New annotation text	0	2026-07-27 10:12:33.310288	2026-07-27 10:12:33.322141	updated title	\N
1369	Updated Title Only	eng	0	novel		0	2026-07-27 10:12:33.334671	2026-07-27 10:12:33.340938	updated title only	\N
1370	Updated Title Empty ISBN	eng	0	novel		0	2026-07-27 10:12:33.352159	2026-07-27 10:12:33.358966	updated title empty isbn	\N
1371	Book One	eng	0	novel		0	2026-07-27 10:12:33.369804	2026-07-27 10:12:33.369804	book one	\N
1372	Book Two	eng	0	novel		0	2026-07-27 10:12:33.376496	2026-07-27 10:12:33.376496	book two	\N
1373	Original Book Title	eng	0	novel		0	2026-07-27 10:12:33.389334	2026-07-27 10:12:33.389334	original book title	\N
1374	Updated Title	eng	0	novel		0	2026-07-27 10:12:33.406755	2026-07-27 10:12:33.413065	updated title	\N
1375	Corrupted ISBN Test	eng	0	novel		0	2026-07-27 10:12:33.422122	2026-07-27 10:12:33.422122	corrupted isbn test	\N
1376	Remove Authors Test	eng	0	novel		0	2026-07-27 10:12:34.585056	2026-07-27 10:12:34.585056	remove authors test	\N
1377	Remove Genres Test	eng	0	novel		0	2026-07-27 10:12:34.604261	2026-07-27 10:12:34.604261	remove genres test	\N
1378	Remove Tags Test	eng	0	novel		0	2026-07-27 10:12:34.620689	2026-07-27 10:12:34.620689	remove tags test	\N
1379	Nil Authors Updated Title	eng	0	novel		0	2026-07-27 10:12:34.638973	2026-07-27 10:12:34.645737	nil authors updated title	\N
1380	Add Author Test	eng	0	novel		0	2026-07-27 10:12:34.655902	2026-07-27 10:12:34.655902	add author test	\N
1381	Test Book Part 1	eng	2023	novel	First test book created via API	0	2026-07-27 10:17:31.94385	2026-07-27 10:17:31.94385	test book part 1	\N
1382	Test Book Part 2	eng	2024	novel	Second test book created via API	0	2026-07-27 10:17:31.951307	2026-07-27 10:17:31.951307	test book part 2	\N
1383	Updated Book Title	eng	2022	novel	Updated description	0	2026-07-27 10:17:31.962338	2026-07-27 10:17:31.968461	updated book title	\N
1385	Updated Title	eng	0	novel	New annotation text	0	2026-07-27 10:17:32.000471	2026-07-27 10:17:32.011347	updated title	\N
1386	Updated Title Only	eng	0	novel		0	2026-07-27 10:17:32.023428	2026-07-27 10:17:32.029888	updated title only	\N
1387	Updated Title Empty ISBN	eng	0	novel		0	2026-07-27 10:17:32.040193	2026-07-27 10:17:32.047425	updated title empty isbn	\N
1388	Book One	eng	0	novel		0	2026-07-27 10:17:32.058987	2026-07-27 10:17:32.058987	book one	\N
1389	Book Two	eng	0	novel		0	2026-07-27 10:17:32.065933	2026-07-27 10:17:32.065933	book two	\N
1390	Original Book Title	eng	0	novel		0	2026-07-27 10:17:32.081033	2026-07-27 10:17:32.081033	original book title	\N
1391	Updated Title	eng	0	novel		0	2026-07-27 10:17:32.100048	2026-07-27 10:17:32.106751	updated title	\N
1392	Corrupted ISBN Test	eng	0	novel		0	2026-07-27 10:17:32.116928	2026-07-27 10:17:32.116928	corrupted isbn test	\N
1393	Remove Authors Test	eng	0	novel		0	2026-07-27 10:17:33.289592	2026-07-27 10:17:33.289592	remove authors test	\N
1394	Remove Genres Test	eng	0	novel		0	2026-07-27 10:17:33.310168	2026-07-27 10:17:33.310168	remove genres test	\N
1395	Remove Tags Test	eng	0	novel		0	2026-07-27 10:17:33.327002	2026-07-27 10:17:33.327002	remove tags test	\N
1396	Nil Authors Updated Title	eng	0	novel		0	2026-07-27 10:17:33.342821	2026-07-27 10:17:33.34881	nil authors updated title	\N
1397	Add Author Test	eng	0	novel		0	2026-07-27 10:17:33.358699	2026-07-27 10:17:33.358699	add author test	\N
1398	Test Book Part 1	eng	2023	novel	First test book created via API	0	2026-07-27 10:21:49.293776	2026-07-27 10:21:49.293776	test book part 1	\N
1399	Test Book Part 2	eng	2024	novel	Second test book created via API	0	2026-07-27 10:21:49.300043	2026-07-27 10:21:49.300043	test book part 2	\N
1400	Updated Book Title	eng	2022	novel	Updated description	0	2026-07-27 10:21:49.310508	2026-07-27 10:21:49.317247	updated book title	\N
1402	Updated Title	eng	0	novel	New annotation text	0	2026-07-27 10:21:49.346495	2026-07-27 10:21:49.356874	updated title	\N
1403	Updated Title Only	eng	0	novel		0	2026-07-27 10:21:49.368984	2026-07-27 10:21:49.375038	updated title only	\N
1404	Updated Title Empty ISBN	eng	0	novel		0	2026-07-27 10:21:49.385398	2026-07-27 10:21:49.391805	updated title empty isbn	\N
1405	Book One	eng	0	novel		0	2026-07-27 10:21:49.402378	2026-07-27 10:21:49.402378	book one	\N
1406	Book Two	eng	0	novel		0	2026-07-27 10:21:49.408026	2026-07-27 10:21:49.408026	book two	\N
1407	Original Book Title	eng	0	novel		0	2026-07-27 10:21:49.420491	2026-07-27 10:21:49.420491	original book title	\N
1408	Updated Title	eng	0	novel		0	2026-07-27 10:21:49.438	2026-07-27 10:21:49.44518	updated title	\N
1409	Corrupted ISBN Test	eng	0	novel		0	2026-07-27 10:21:49.455955	2026-07-27 10:21:49.455955	corrupted isbn test	\N
1410	Remove Authors Test	eng	0	novel		0	2026-07-27 10:21:50.599706	2026-07-27 10:21:50.599706	remove authors test	\N
1411	Remove Genres Test	eng	0	novel		0	2026-07-27 10:21:50.619964	2026-07-27 10:21:50.619964	remove genres test	\N
1412	Remove Tags Test	eng	0	novel		0	2026-07-27 10:21:50.639665	2026-07-27 10:21:50.639665	remove tags test	\N
1413	Nil Authors Updated Title	eng	0	novel		0	2026-07-27 10:21:50.656015	2026-07-27 10:21:50.662517	nil authors updated title	\N
1414	Add Author Test	eng	0	novel		0	2026-07-27 10:21:50.673931	2026-07-27 10:21:50.673931	add author test	\N
\.


--
-- Name: collection_items_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('public.collection_items_id_seq', 1, false);


--
-- Name: collections_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('public.collections_id_seq', 1, false);


--
-- Name: conversion_log_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('public.conversion_log_id_seq', 1, false);


--
-- Name: duplicate_candidates_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('public.duplicate_candidates_id_seq', 1, false);


--
-- Name: edition_files_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('public.edition_files_id_seq', 1422, true);


--
-- Name: editions_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('public.editions_id_seq', 1422, true);


--
-- Name: formats_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('public.formats_id_seq', 16, true);


--
-- Name: genres_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('public.genres_id_seq', 340, true);


--
-- Name: import_sessions_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('public.import_sessions_id_seq', 1, false);


--
-- Name: persons_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('public.persons_id_seq', 1711, true);


--
-- Name: reading_progress_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('public.reading_progress_id_seq', 1, false);


--
-- Name: refresh_tokens_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('public.refresh_tokens_id_seq', 138, true);


--
-- Name: shelf_tokens_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('public.shelf_tokens_id_seq', 20, true);


--
-- Name: tags_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('public.tags_id_seq', 66, true);


--
-- Name: toc_entries_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('public.toc_entries_id_seq', 1, false);


--
-- Name: user_books_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('public.user_books_id_seq', 74, true);


--
-- Name: user_devices_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('public.user_devices_id_seq', 95, true);


--
-- Name: users_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('public.users_id_seq', 1094, true);


--
-- Name: works_id_seq; Type: SEQUENCE SET; Schema: public; Owner: -
--

SELECT pg_catalog.setval('public.works_id_seq', 1414, true);


--
-- Name: collection_items collection_items_collection_id_edition_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.collection_items
    ADD CONSTRAINT collection_items_collection_id_edition_id_key UNIQUE (collection_id, edition_id);


--
-- Name: collection_items collection_items_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.collection_items
    ADD CONSTRAINT collection_items_pkey PRIMARY KEY (id);


--
-- Name: collections collections_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.collections
    ADD CONSTRAINT collections_pkey PRIMARY KEY (id);


--
-- Name: conversion_log conversion_log_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.conversion_log
    ADD CONSTRAINT conversion_log_pkey PRIMARY KEY (id);


--
-- Name: duplicate_candidates duplicate_candidates_edition_id_1_edition_id_2_match_type_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.duplicate_candidates
    ADD CONSTRAINT duplicate_candidates_edition_id_1_edition_id_2_match_type_key UNIQUE (edition_id_1, edition_id_2, match_type);


--
-- Name: duplicate_candidates duplicate_candidates_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.duplicate_candidates
    ADD CONSTRAINT duplicate_candidates_pkey PRIMARY KEY (id);


--
-- Name: edition_files edition_files_edition_id_format_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.edition_files
    ADD CONSTRAINT edition_files_edition_id_format_id_key UNIQUE (edition_id, format_id);


--
-- Name: edition_files edition_files_file_hash_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.edition_files
    ADD CONSTRAINT edition_files_file_hash_key UNIQUE (file_hash);


--
-- Name: edition_files edition_files_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.edition_files
    ADD CONSTRAINT edition_files_pkey PRIMARY KEY (id);


--
-- Name: edition_tags edition_tags_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.edition_tags
    ADD CONSTRAINT edition_tags_pkey PRIMARY KEY (edition_id, tag_id);


--
-- Name: editions editions_isbn_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.editions
    ADD CONSTRAINT editions_isbn_key UNIQUE (isbn);


--
-- Name: editions editions_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.editions
    ADD CONSTRAINT editions_pkey PRIMARY KEY (id);


--
-- Name: formats formats_name_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.formats
    ADD CONSTRAINT formats_name_key UNIQUE (name);


--
-- Name: formats formats_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.formats
    ADD CONSTRAINT formats_pkey PRIMARY KEY (id);


--
-- Name: genres genres_name_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.genres
    ADD CONSTRAINT genres_name_key UNIQUE (name);


--
-- Name: genres genres_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.genres
    ADD CONSTRAINT genres_pkey PRIMARY KEY (id);


--
-- Name: import_sessions import_sessions_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.import_sessions
    ADD CONSTRAINT import_sessions_pkey PRIMARY KEY (id);


--
-- Name: languages languages_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.languages
    ADD CONSTRAINT languages_pkey PRIMARY KEY (code);


--
-- Name: persons persons_first_name_last_name_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.persons
    ADD CONSTRAINT persons_first_name_last_name_key UNIQUE (first_name, last_name);


--
-- Name: persons persons_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.persons
    ADD CONSTRAINT persons_pkey PRIMARY KEY (id);


--
-- Name: read_list read_list_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.read_list
    ADD CONSTRAINT read_list_pkey PRIMARY KEY (id);


--
-- Name: reading_progress reading_progress_edition_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.reading_progress
    ADD CONSTRAINT reading_progress_edition_id_key UNIQUE (edition_id);


--
-- Name: reading_progress reading_progress_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.reading_progress
    ADD CONSTRAINT reading_progress_pkey PRIMARY KEY (id);


--
-- Name: refresh_tokens refresh_tokens_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.refresh_tokens
    ADD CONSTRAINT refresh_tokens_pkey PRIMARY KEY (id);


--
-- Name: refresh_tokens refresh_tokens_token_hash_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.refresh_tokens
    ADD CONSTRAINT refresh_tokens_token_hash_key UNIQUE (token_hash);


--
-- Name: settings settings_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.settings
    ADD CONSTRAINT settings_pkey PRIMARY KEY (key);


--
-- Name: shelf_tokens shelf_tokens_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.shelf_tokens
    ADD CONSTRAINT shelf_tokens_pkey PRIMARY KEY (id);


--
-- Name: shelf_tokens shelf_tokens_token_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.shelf_tokens
    ADD CONSTRAINT shelf_tokens_token_key UNIQUE (token);


--
-- Name: tags tags_name_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.tags
    ADD CONSTRAINT tags_name_key UNIQUE (name);


--
-- Name: tags tags_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.tags
    ADD CONSTRAINT tags_pkey PRIMARY KEY (id);


--
-- Name: toc_entries toc_entries_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.toc_entries
    ADD CONSTRAINT toc_entries_pkey PRIMARY KEY (id);


--
-- Name: user_books user_books_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_books
    ADD CONSTRAINT user_books_pkey PRIMARY KEY (id);


--
-- Name: user_books user_books_user_id_edition_id_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_books
    ADD CONSTRAINT user_books_user_id_edition_id_key UNIQUE (user_id, edition_id);


--
-- Name: user_devices user_devices_device_fingerprint_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_devices
    ADD CONSTRAINT user_devices_device_fingerprint_key UNIQUE (device_fingerprint);


--
-- Name: user_devices user_devices_device_name_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_devices
    ADD CONSTRAINT user_devices_device_name_key UNIQUE (device_name);


--
-- Name: user_devices user_devices_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_devices
    ADD CONSTRAINT user_devices_pkey PRIMARY KEY (id);


--
-- Name: users users_email_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.users
    ADD CONSTRAINT users_email_key UNIQUE (email);


--
-- Name: users users_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.users
    ADD CONSTRAINT users_pkey PRIMARY KEY (id);


--
-- Name: users users_username_key; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.users
    ADD CONSTRAINT users_username_key UNIQUE (username);


--
-- Name: work_contributors work_contributors_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.work_contributors
    ADD CONSTRAINT work_contributors_pkey PRIMARY KEY (work_id, person_id, role);


--
-- Name: work_genres work_genres_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.work_genres
    ADD CONSTRAINT work_genres_pkey PRIMARY KEY (work_id, genre_id);


--
-- Name: works works_pkey; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.works
    ADD CONSTRAINT works_pkey PRIMARY KEY (id);


--
-- Name: idx_duplicate_candidates_unconfirmed; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_duplicate_candidates_unconfirmed ON public.duplicate_candidates USING btree (edition_id_1) WHERE (is_confirmed = false);


--
-- Name: idx_edition_files_edition; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_edition_files_edition ON public.edition_files USING btree (edition_id);


--
-- Name: idx_edition_files_format; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_edition_files_format ON public.edition_files USING btree (format_id);


--
-- Name: idx_edition_files_hash; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_edition_files_hash ON public.edition_files USING btree (file_hash) WHERE (file_hash IS NOT NULL);


--
-- Name: idx_editions_fts; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_editions_fts ON public.editions USING gin (search_vector);


--
-- Name: idx_editions_isbn; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_editions_isbn ON public.editions USING btree (isbn) WHERE (isbn IS NOT NULL);


--
-- Name: idx_editions_language; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_editions_language ON public.editions USING btree (language);


--
-- Name: idx_editions_lower_title; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_editions_lower_title ON public.editions USING gin (lower_title public.gin_trgm_ops);


--
-- Name: idx_editions_title_trgm; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_editions_title_trgm ON public.editions USING gin (title public.gin_trgm_ops);


--
-- Name: idx_editions_uploaded_by; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_editions_uploaded_by ON public.editions USING btree (uploaded_by);


--
-- Name: idx_editions_work; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_editions_work ON public.editions USING btree (work_id);


--
-- Name: idx_editions_year; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_editions_year ON public.editions USING btree (year);


--
-- Name: idx_persons_first_trgm; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_persons_first_trgm ON public.persons USING gin (first_name public.gin_trgm_ops);


--
-- Name: idx_persons_last_trgm; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_persons_last_trgm ON public.persons USING gin (last_name public.gin_trgm_ops);


--
-- Name: idx_persons_lower_fio; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_persons_lower_fio ON public.persons USING gin (lower_fio public.gin_trgm_ops);


--
-- Name: idx_read_list_listname; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_read_list_listname ON public.read_list USING btree (listname);


--
-- Name: idx_read_list_new_id; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX idx_read_list_new_id ON public.read_list USING btree (id);


--
-- Name: idx_read_list_user_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_read_list_user_id ON public.read_list USING btree (user_id);


--
-- Name: idx_reading_progress_rating; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_reading_progress_rating ON public.reading_progress USING btree (rating) WHERE (rating IS NOT NULL);


--
-- Name: idx_refresh_tokens_hash; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_refresh_tokens_hash ON public.refresh_tokens USING btree (token_hash);


--
-- Name: idx_refresh_tokens_user; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_refresh_tokens_user ON public.refresh_tokens USING btree (user_id);


--
-- Name: idx_series_trgm; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_series_trgm ON public.editions USING gin (series public.gin_trgm_ops);


--
-- Name: idx_shelf_tokens_edition; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_shelf_tokens_edition ON public.shelf_tokens USING btree (edition_id);


--
-- Name: idx_shelf_tokens_token; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_shelf_tokens_token ON public.shelf_tokens USING btree (token);


--
-- Name: idx_toc_edition; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_toc_edition ON public.toc_entries USING btree (edition_id);


--
-- Name: idx_toc_title_trgm; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_toc_title_trgm ON public.toc_entries USING gin (title public.gin_trgm_ops);


--
-- Name: idx_user_books_edition_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_user_books_edition_id ON public.user_books USING btree (edition_id);


--
-- Name: idx_user_books_status; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_user_books_status ON public.user_books USING btree (status);


--
-- Name: idx_user_books_user_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_user_books_user_id ON public.user_books USING btree (user_id);


--
-- Name: idx_users_email; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_users_email ON public.users USING btree (email);


--
-- Name: idx_users_username; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_users_username ON public.users USING btree (username);


--
-- Name: idx_work_contributors_person; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_work_contributors_person ON public.work_contributors USING btree (person_id);


--
-- Name: idx_work_contributors_work; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_work_contributors_work ON public.work_contributors USING btree (work_id);


--
-- Name: idx_works_fts; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_works_fts ON public.works USING gin (search_vector);


--
-- Name: idx_works_lower_title; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_works_lower_title ON public.works USING gin (lower_original_title public.gin_trgm_ops);


--
-- Name: idx_works_title_trgm; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX idx_works_title_trgm ON public.works USING gin (original_title public.gin_trgm_ops);


--
-- Name: editions trg_editions_normalize; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER trg_editions_normalize BEFORE INSERT OR UPDATE ON public.editions FOR EACH ROW EXECUTE FUNCTION public.normalize_search_field();


--
-- Name: editions trg_editions_updated; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER trg_editions_updated BEFORE UPDATE ON public.editions FOR EACH ROW EXECUTE FUNCTION public.update_timestamp();


--
-- Name: edition_files trg_files_updated; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER trg_files_updated BEFORE UPDATE ON public.edition_files FOR EACH ROW EXECUTE FUNCTION public.update_timestamp();


--
-- Name: persons trg_persons_normalize; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER trg_persons_normalize BEFORE INSERT OR UPDATE ON public.persons FOR EACH ROW EXECUTE FUNCTION public.normalize_search_field();


--
-- Name: read_list trg_readlist_sync_status; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER trg_readlist_sync_status AFTER UPDATE OF status ON public.read_list FOR EACH ROW EXECUTE FUNCTION public.sync_readlist_status_to_userbooks();


--
-- Name: user_books trg_userbooks_sync_status; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER trg_userbooks_sync_status AFTER UPDATE OF status ON public.user_books FOR EACH ROW EXECUTE FUNCTION public.sync_userbooks_status_to_readlist();


--
-- Name: works trg_works_normalize; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER trg_works_normalize BEFORE INSERT OR UPDATE ON public.works FOR EACH ROW EXECUTE FUNCTION public.normalize_search_field();


--
-- Name: works trg_works_updated; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER trg_works_updated BEFORE UPDATE ON public.works FOR EACH ROW EXECUTE FUNCTION public.update_timestamp();


--
-- Name: collection_items collection_items_collection_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.collection_items
    ADD CONSTRAINT collection_items_collection_id_fkey FOREIGN KEY (collection_id) REFERENCES public.collections(id) ON DELETE CASCADE;


--
-- Name: collection_items collection_items_edition_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.collection_items
    ADD CONSTRAINT collection_items_edition_id_fkey FOREIGN KEY (edition_id) REFERENCES public.editions(id) ON DELETE CASCADE;


--
-- Name: conversion_log conversion_log_source_file_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.conversion_log
    ADD CONSTRAINT conversion_log_source_file_id_fkey FOREIGN KEY (source_file_id) REFERENCES public.edition_files(id) ON DELETE SET NULL;


--
-- Name: conversion_log conversion_log_target_file_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.conversion_log
    ADD CONSTRAINT conversion_log_target_file_id_fkey FOREIGN KEY (target_file_id) REFERENCES public.edition_files(id) ON DELETE SET NULL;


--
-- Name: duplicate_candidates duplicate_candidates_edition_id_1_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.duplicate_candidates
    ADD CONSTRAINT duplicate_candidates_edition_id_1_fkey FOREIGN KEY (edition_id_1) REFERENCES public.editions(id) ON DELETE CASCADE;


--
-- Name: duplicate_candidates duplicate_candidates_edition_id_2_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.duplicate_candidates
    ADD CONSTRAINT duplicate_candidates_edition_id_2_fkey FOREIGN KEY (edition_id_2) REFERENCES public.editions(id) ON DELETE CASCADE;


--
-- Name: edition_files edition_files_edition_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.edition_files
    ADD CONSTRAINT edition_files_edition_id_fkey FOREIGN KEY (edition_id) REFERENCES public.editions(id) ON DELETE CASCADE;


--
-- Name: edition_files edition_files_format_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.edition_files
    ADD CONSTRAINT edition_files_format_id_fkey FOREIGN KEY (format_id) REFERENCES public.formats(id) ON DELETE RESTRICT;


--
-- Name: edition_files edition_files_source_file_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.edition_files
    ADD CONSTRAINT edition_files_source_file_id_fkey FOREIGN KEY (source_file_id) REFERENCES public.edition_files(id);


--
-- Name: edition_tags edition_tags_edition_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.edition_tags
    ADD CONSTRAINT edition_tags_edition_id_fkey FOREIGN KEY (edition_id) REFERENCES public.editions(id) ON DELETE CASCADE;


--
-- Name: edition_tags edition_tags_tag_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.edition_tags
    ADD CONSTRAINT edition_tags_tag_id_fkey FOREIGN KEY (tag_id) REFERENCES public.tags(id) ON DELETE CASCADE;


--
-- Name: editions editions_language_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.editions
    ADD CONSTRAINT editions_language_fkey FOREIGN KEY (language) REFERENCES public.languages(code);


--
-- Name: editions editions_uploaded_by_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.editions
    ADD CONSTRAINT editions_uploaded_by_fkey FOREIGN KEY (uploaded_by) REFERENCES public.users(id) ON DELETE SET NULL;


--
-- Name: editions editions_work_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.editions
    ADD CONSTRAINT editions_work_id_fkey FOREIGN KEY (work_id) REFERENCES public.works(id) ON DELETE CASCADE;


--
-- Name: genres genres_parent_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.genres
    ADD CONSTRAINT genres_parent_id_fkey FOREIGN KEY (parent_id) REFERENCES public.genres(id);


--
-- Name: read_list read_list_author_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.read_list
    ADD CONSTRAINT read_list_author_id_fkey FOREIGN KEY (author_id) REFERENCES public.persons(id) ON DELETE SET NULL;


--
-- Name: read_list read_list_book_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.read_list
    ADD CONSTRAINT read_list_book_id_fkey FOREIGN KEY (book_id) REFERENCES public.editions(id) ON DELETE SET NULL;


--
-- Name: read_list read_list_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.read_list
    ADD CONSTRAINT read_list_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE CASCADE;


--
-- Name: reading_progress reading_progress_edition_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.reading_progress
    ADD CONSTRAINT reading_progress_edition_id_fkey FOREIGN KEY (edition_id) REFERENCES public.editions(id) ON DELETE CASCADE;


--
-- Name: refresh_tokens refresh_tokens_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.refresh_tokens
    ADD CONSTRAINT refresh_tokens_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE CASCADE;


--
-- Name: shelf_tokens shelf_tokens_edition_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.shelf_tokens
    ADD CONSTRAINT shelf_tokens_edition_id_fkey FOREIGN KEY (edition_id) REFERENCES public.editions(id) ON DELETE CASCADE;


--
-- Name: toc_entries toc_entries_edition_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.toc_entries
    ADD CONSTRAINT toc_entries_edition_id_fkey FOREIGN KEY (edition_id) REFERENCES public.editions(id) ON DELETE CASCADE;


--
-- Name: toc_entries toc_entries_parent_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.toc_entries
    ADD CONSTRAINT toc_entries_parent_id_fkey FOREIGN KEY (parent_id) REFERENCES public.toc_entries(id) ON DELETE CASCADE;


--
-- Name: user_books user_books_edition_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_books
    ADD CONSTRAINT user_books_edition_id_fkey FOREIGN KEY (edition_id) REFERENCES public.editions(id) ON DELETE CASCADE;


--
-- Name: user_books user_books_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_books
    ADD CONSTRAINT user_books_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE CASCADE;


--
-- Name: user_devices user_devices_user_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_devices
    ADD CONSTRAINT user_devices_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE CASCADE;


--
-- Name: work_contributors work_contributors_person_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.work_contributors
    ADD CONSTRAINT work_contributors_person_id_fkey FOREIGN KEY (person_id) REFERENCES public.persons(id) ON DELETE CASCADE;


--
-- Name: work_contributors work_contributors_work_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.work_contributors
    ADD CONSTRAINT work_contributors_work_id_fkey FOREIGN KEY (work_id) REFERENCES public.works(id) ON DELETE CASCADE;


--
-- Name: work_genres work_genres_genre_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.work_genres
    ADD CONSTRAINT work_genres_genre_id_fkey FOREIGN KEY (genre_id) REFERENCES public.genres(id) ON DELETE CASCADE;


--
-- Name: work_genres work_genres_work_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.work_genres
    ADD CONSTRAINT work_genres_work_id_fkey FOREIGN KEY (work_id) REFERENCES public.works(id) ON DELETE CASCADE;


--
-- Name: works works_original_language_fkey; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.works
    ADD CONSTRAINT works_original_language_fkey FOREIGN KEY (original_language) REFERENCES public.languages(code);


--
-- PostgreSQL database dump complete
--

\unrestrict O3uU6yjLFzTx7YUO66ZUwYzYfeElcujj6zo4tr8Hdf1pviNlZsoI02alJjSq0S9

