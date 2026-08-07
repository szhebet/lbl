-- Migration 4.8: Prefill genres.ru_name with Russian display names.
--
-- Maps FB2 genre codes to their Russian names. Idempotent:
--   * record absent  -> INSERT with the Russian name;
--   * record present -> UPDATE only if ru_name is NULL or empty
--                       (already-filled values are preserved).
INSERT INTO genres (name, ru_name)
VALUES
    ('management', 'Менеджмент'),
    ('prose_contemporary', 'Современная проза'),
    ('adv_indian', 'Приключения про индейцев'),
    ('sf_social', 'Социальная фантастика'),
    ('nonf_publicism', 'Публицистика'),
    ('adv_history', 'Исторические приключения'),
    ('foreign_adventure', 'Зарубежные приключения'),
    ('literature_19', 'Литература XIX века'),
    ('foreign_prose', 'Зарубежная проза'),
    ('adv_maritime', 'Морские приключения'),
    ('literature_20', 'Литература XX века'),
    ('prose_rus_classic', 'Русская классическая проза'),
    ('child_education', 'Детская образовательная литература'),
    ('sci_business', 'Деловая литература'),
    ('nonf_biography', 'Биографии и мемуары'),
    ('economics', 'Экономика'),
    ('foreign_edu', 'Зарубежная образовательная литература'),
    ('sci_philosophy', 'Философия'),
    ('religion_esoterics', 'Религия и эзотерика'),
    ('sci_politics', 'Политика'),
    ('religion_self', 'Самосовершенствование'),
    ('russian_contemporary', 'Современная русская проза'),
    ('sci_psychology', 'Психология'),
    ('prose_classic', 'Классическая проза'),
    ('thriller', 'Триллер'),
    ('popular_business', 'Популярная бизнес-литература'),
    ('sf', 'Научная фантастика'),
    ('sf_humor', 'Юмористическая фантастика'),
    ('religion_rel', 'Религиозная литература'),
    ('antique_east', 'Древневосточная литература'),
    ('nonf_criticism', 'Критика'),
    ('prose_counter', 'Контркультура'),
    ('antique_ant', 'Античная литература'),
    ('religion', 'Религия'),
    ('DetectiveGenreTest', 'Детектив')
ON CONFLICT (name) DO UPDATE SET ru_name = CASE
    WHEN NULLIF(genres.ru_name, '') IS NULL THEN EXCLUDED.ru_name
    ELSE genres.ru_name
END;
