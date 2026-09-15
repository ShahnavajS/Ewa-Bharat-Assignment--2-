-- The Go API is the only public data boundary. Supabase's REST roles must not
-- gain direct table access if Data API auto-exposure is enabled on the project.
DO $$
DECLARE
    table_name TEXT;
    role_name TEXT;
BEGIN
    FOREACH table_name IN ARRAY ARRAY['windows', 'media_items', 'playlist_items', 'sync_events']
    LOOP
        EXECUTE format('ALTER TABLE public.%I ENABLE ROW LEVEL SECURITY', table_name);
        FOREACH role_name IN ARRAY ARRAY['anon', 'authenticated']
        LOOP
            IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname = role_name) THEN
                EXECUTE format('REVOKE ALL ON TABLE public.%I FROM %I', table_name, role_name);
            END IF;
        END LOOP;
    END LOOP;
END
$$;
