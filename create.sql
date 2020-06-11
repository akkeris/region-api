do $$
begin
    create table if not exists appbindings
    (
        appname TEXT NOT NULL,
        bindtype TEXT NOT NULL,
        bindname TEXT NOT NULL,
        space TEXT NOT NULL,
        CONSTRAINT appbindings_pkey PRIMARY KEY (appname, bindtype, bindname, space)
    );

    create table if not exists configvarsmap (
        appname TEXT NOT NULL,
        bindtype TEXT NOT NULL,
        bindname TEXT NOT NULL,
        space TEXT NOT NULL,
        mapid UUID primary key not null,
        varname text not null,
        newname text not null,
        action text not null -- delete, rename, copy
    );

    create unique index if not exists appbindings_appname_bindtype_bindname_space_key ON appbindings (appname, bindtype, bindname, space);

    create table if not exists appfeature
    (
        space TEXT NOT NULL,
        app TEXT NOT NULL,
        optionkey TEXT NOT NULL,
        optionvalue BOOLEAN,
        CONSTRAINT appfeature_pkey PRIMARY KEY (space, app, optionkey)
    );

    create table if not exists service_instances
    (
        instance_id varchar(1024) not null primary key,
        service_id varchar(1024) not null,
        plan_id varchar(1024) not null,
        operation_key varchar(1024) null,
        status varchar(1024) not null default '',
        metadata text default ''
    );

    create table if not exists appopsgenie
    (
        space TEXT NOT NULL,
        app TEXT NOT NULL,
        optionvalue BOOLEAN,
        CONSTRAINT appopsgenie_pkey PRIMARY KEY (space, app)
    );

    create table if not exists apps
    (
        appid UUID PRIMARY KEY NOT NULL,
        name TEXT,
        port INTEGER,
        instances INTEGER
    );

    create unique index if not exists unique_appname ON apps (name);
    
    create table if not exists callbacks
    (
        space TEXT NOT NULL,
        appname TEXT NOT NULL,
        tag TEXT NOT NULL,
        method TEXT NOT NULL,
        url TEXT,
        CONSTRAINT callbacks_pkey PRIMARY KEY (space, appname, tag, method)
    );

    create table if not exists certs
    (
        id UUID PRIMARY KEY NOT NULL,
        request TEXT,
        ordernumber TEXT,
        cn TEXT,
        san TEXT
    );

    create unique index if not exists certs_request_ordernumber_cn_key ON certs (request, ordernumber, cn);

    create table if not exists configvars
    (
        setname TEXT NOT NULL,
        varname TEXT NOT NULL,
        varvalue TEXT,
        CONSTRAINT configvars_pk PRIMARY KEY (setname, varname)
    );

    create table if not exists cronjobs
    (
        name TEXT NOT NULL,
        space TEXT NOT NULL,
        cmd TEXT,
        schedule TEXT NOT NULL,
        plan TEXT,
        CONSTRAINT cronjobs_pkey PRIMARY KEY (name, space)
    );

    create table if not exists includes
    (
        parent TEXT NOT NULL,
        child TEXT NOT NULL,
        CONSTRAINT includes_pk PRIMARY KEY (parent, child)
    );

    create table if not exists jobs
    (
        name TEXT NOT NULL,
        space TEXT NOT NULL,
        cmd TEXT,
        plan TEXT,
        CONSTRAINT "jobs-pkey" PRIMARY KEY (name, space)
    );

    create table if not exists plans
    (
        name TEXT PRIMARY KEY NOT NULL,
        memrequest TEXT,
        memlimit TEXT,
        price INTEGER,
        deprecated BOOLEAN DEFAULT FALSE,
        "description" TEXT,
        "type" TEXT
    );

    ALTER TABLE plans 
        ADD COLUMN IF NOT EXISTS deprecated BOOLEAN DEFAULT FALSE,
        ADD COLUMN IF NOT EXISTS "description" TEXT,
        ADD COLUMN IF NOT EXISTS "type" TEXT;

    create table if not exists routerpaths
    (
        domain TEXT NOT NULL,
        path TEXT NOT NULL,
        space TEXT NOT NULL,
        app TEXT NOT NULL,
        replacepath TEXT,
        CONSTRAINT routerpaths_pkey PRIMARY KEY (domain, path, space, app)
    );

    if not exists (SELECT NULL 
              FROM INFORMATION_SCHEMA.COLUMNS
             WHERE table_name = 'routerpaths'
              AND column_name = 'filters'
              and table_schema = 'public') then
        alter table routerpaths add column filters text;
    end if; 

    create table if not exists routers
    (
        routerid UUID PRIMARY KEY NOT NULL,
        domain TEXT,
        internal BOOLEAN
    );

    create unique index if not exists routers_domain_key ON routers (domain);

    create table if not exists sets
    (
        setid UUID PRIMARY KEY NOT NULL,
        name TEXT,
        type TEXT
    );

    create unique index if not exists unique_name ON sets (name);

    create table if not exists spaces
    (
        name TEXT PRIMARY KEY NOT NULL,
        internal BOOLEAN DEFAULT false,
        compliancetags TEXT,
        stack text not null
    );

    create table if not exists stacks 
    (
        stack text primary key not null,
        description text not null,
        api_server text not null,
        api_version text not null,
        image_pull_secret text not null,
        auth_type text not null,
        auth_vault_path text not null
    );

    create unique index if not exists unique_name ON stacks (stack);

    if not exists (SELECT NULL 
              FROM INFORMATION_SCHEMA.COLUMNS
             WHERE table_name = 'spaces'
              AND column_name = 'stack'
              and table_schema = 'public') then
        alter table spaces add column stack text;
    end if;

    create table if not exists spacesapps
    (
        space TEXT NOT NULL,
        appname TEXT NOT NULL,
        instances INTEGER,
        plan TEXT,
        healthcheck TEXT,
        CONSTRAINT spacesapps_pkey PRIMARY KEY (space, appname)
    );

    create table if not exists subscribers
    (
        space TEXT NOT NULL,
        app TEXT NOT NULL,
        email TEXT NOT NULL,
        CONSTRAINT subscribers_pkey PRIMARY KEY (space, app, email)
    );

    if (select count(*) from plans) = 0 then
        INSERT INTO public.plans (name, memrequest, memlimit, price, "description") 
            VALUES ('scout', '256Mi', '256Mi', 10, '256MB RAM, 2.4 GHz Intel Xeon E5-2676 v3 CPU, 750Mbps Networking');
        INSERT INTO public.plans (name, memrequest, memlimit, price, "description") 
            VALUES ('scout-prod', '256Mi', '256Mi', 15, '256MB RAM, 2.4 GHz Intel Xeon E5-2676 v3 CPU, 750Mbps Networking');
        INSERT INTO public.plans (name, memrequest, memlimit, price, "description") 
            VALUES ('constellation', '256Mi', '512Mi', 20, '256MB RAM, 2.4 GHz Intel Xeon E5-2676 v3 CPU, 750Mbps Networking');
        INSERT INTO public.plans (name, memrequest, memlimit, price, "description") 
            VALUES ('constellation-prod', '512Mi', '512Mi', 25, '512MB RAM, 2.4 GHz Intel Xeon E5-2676 v3 CPU, 750Mbps Networking');
        INSERT INTO public.plans (name, memrequest, memlimit, price, "description") 
            VALUES ('akira', '768Mi', '1024Mi', 30, '768MB RAM, 2.4 GHz Intel Xeon E5-2676 v3 CPU, 750Mbps Networking');
        INSERT INTO public.plans (name, memrequest, memlimit, price, "description") 
            VALUES ('akira-prod', '1024Mi', '1024Mi', 35, '1024MB RAM, 2.4 GHz Intel Xeon E5-2676 v3 CPU, 750Mbps Networking');
        INSERT INTO public.plans (name, memrequest, memlimit, price, "description") 
            VALUES ('galaxy', '1024Mi', '1536Mi', 40, '1024MB RAM, 2.4 GHz Intel Xeon E5-2676 v3 CPU, 750Mbps Networking');
        INSERT INTO public.plans (name, memrequest, memlimit, price, "description") 
            VALUES ('galaxy-prod', '1536Mi', '1536Mi', 45, '1536MB RAM, 2.4 GHz Intel Xeon E5-2676 v3 CPU, 750Mbps Networking');
        INSERT INTO public.plans (name, memrequest, memlimit, price, "description") 
            VALUES ('sovereign', '1536Mi', '2048Mi', 50, '1536MB RAM, 2.4 GHz Intel Xeon E5-2676 v3 CPU, 750Mbps Networking');
        INSERT INTO public.plans (name, memrequest, memlimit, price, "description") 
            VALUES ('sovereign-prod', '2048Mi', '2048Mi', 55, '2048MB RAM, 2.4 GHz Intel Xeon E5-2676 v3 CPU, 750Mbps Networking');
    end if;

    if (select count(*) from plans where name = 'scout' and description is null) > 0 then
        update plans set description = '256MB RAM, 2.4 GHz Intel Xeon E5-2676 v3 CPU, 750Mbps Networking' where description is null and name = 'scout';
    end if;

    if (select count(*) from plans where name = 'scout-prod' and description is null) > 0 then
        update plans set description = '256MB RAM, 2.4 GHz Intel Xeon E5-2676 v3 CPU, 750Mbps Networking' where description is null and name = 'scout-prod';
    end if;

    if (select count(*) from plans where name = 'constellation' and description is null) > 0 then
        update plans set description = '256MB RAM, 2.4 GHz Intel Xeon E5-2676 v3 CPU, 750Mbps Networking' where description is null and name = 'constellation';
    end if;

    if (select count(*) from plans where name = 'constellation-prod' and description is null) > 0 then
        update plans set description = '512MB RAM, 2.4 GHz Intel Xeon E5-2676 v3 CPU, 750Mbps Networking' where description is null and name = 'constellation-prod';
    end if;

    if (select count(*) from plans where name = 'akira' and description is null) > 0 then
        update plans set description = '768MB RAM, 2.4 GHz Intel Xeon E5-2676 v3 CPU, 750Mbps Networking' where description is null and name = 'akira';
    end if;

    if (select count(*) from plans where name = 'akira-prod' and description is null) > 0 then
        update plans set description = '1024MB RAM, 2.4 GHz Intel Xeon E5-2676 v3 CPU, 750Mbps Networking' where description is null and name = 'akira-prod';
    end if;

    if (select count(*) from plans where name = 'galaxy' and description is null) > 0 then
        update plans set description = '1024MB RAM, 2.4 GHz Intel Xeon E5-2676 v3 CPU, 750Mbps Networking' where description is null and name = 'galaxy';
    end if;

    if (select count(*) from plans where name = 'galaxy-prod' and description is null) > 0 then
        update plans set description = '1536MB RAM, 2.4 GHz Intel Xeon E5-2676 v3 CPU, 750Mbps Networking' where description is null and name = 'galaxy-prod';
    end if;

    if (select count(*) from plans where name = 'sovereign' and description is null) > 0 then
        update plans set description = '1536MB RAM, 2.4 GHz Intel Xeon E5-2676 v3 CPU, 750Mbps Networking' where description is null and name = 'sovereign';
    end if;

    if (select count(*) from plans where name = 'sovereign-prod' and description is null) > 0 then
        update plans set description = '2048MB RAM, 2.4 GHz Intel Xeon E5-2676 v3 CPU, 750Mbps Networking' where description is null and name = 'sovereign-prod';
    end if;


    if (select count(*) from sets where name='oct-apitest-cs') = 0 then
        insert into sets (setid, name, type) values ('48594a81-fa86-4a2e-5284-6aa9f87d319f', 'oct-apitest-cs', 'config');
    end if;

    if (select count(*) from configvars where setname='oct-apitest-cs') = 0 then
        insert into configvars (setname, varname, varvalue) values ('oct-apitest-cs', 'testvar', 'testvalue');
        insert into configvars (setname, varname, varvalue) values ('oct-apitest-cs', 'testvar2', 'testval2');
        insert into configvars (setname, varname, varvalue) values ('oct-apitest-cs', 'METADATA_URL', 'http://169.254.169.254/latest/meta-data/placement/availability-zone');
    end if;

    if (select count(*) from spaces where name='default') = 0 then
        insert into spaces (name, internal, compliancetags, stack) values ('default', false, 'prod', 'ds1');
    end if;
    if (select count(*) from spaces where name='gotest') = 0 then
        insert into spaces (name, internal, compliancetags, stack) values ('gotest', false, '', 'ds1');
    end if;
    if (select count(*) from spaces where name='deck1') = 0 then
        insert into spaces (name, internal, compliancetags, stack) values ('deck1', false, '', 'ds1');
    end if;
    if (select count(*) from apps where name='oct-apitest') = 0 then
        insert into apps (appid, name, port, instances) values ('6fcd46c6-e3ce-411b-602a-492592c2ee22','oct-apitest', 3000, null);
        insert into spacesapps(space,appname,instances,plan,healthcheck) values ('deck1','oct-apitest',1,'hobby',null);
    end if;

    -- Add "appid" column to spacesapps table
    if not exists 
    (
        SELECT NULL FROM INFORMATION_SCHEMA.COLUMNS
            WHERE table_name = 'spacesapps'
            AND column_name = 'appid'
            and table_schema = 'public'
    ) then
        alter table spacesapps add column appid UUID;
    end if; 

    -- Add "port" column to spacesapps table
    if not exists 
    (
        SELECT NULL FROM INFORMATION_SCHEMA.COLUMNS
            WHERE table_name = 'spacesapps'
            AND column_name = 'port'
            and table_schema = 'public'
    ) then
        alter table spacesapps add column port integer;
    end if;

    -- =================================================
    -- Foreign Data Wrapper to Controller API Database
    -- 
    -- NOTE:
    --   Temporary measure that will be removed
    --   when the v2 schema migration is complete.
    -- =================================================

    -- Make sure postgres_fdw extension exists
    create extension if not exists postgres_fdw;

    -- Create neccesary controller-api custom data types
    if not exists (select 1 from pg_type where typname = 'alpha_numeric') then
      create domain alpha_numeric as varchar(128) check (value ~ '^[A-z0-9\-]+$');
    end if;

    if not exists (select 1 from pg_type where typname = 'href') then
        create domain href as varchar(1024);
    end if;

    -- Create fdw server if not exists
    --  $1: controller-api database host
    --  $2: controller-api database name
    if not exists (
        select null from pg_foreign_server
            join pg_foreign_data_wrapper pgfdw on pgfdw.oid = srvfdw
            where fdwname = 'postgres_fdw' and srvname = 'controller_api_server'
    ) then
        create server controller_api_server
            foreign data wrapper postgres_fdw options (host $1, dbname $2);
    end if;

    -- Create fdw user mapping if not exists
    --  $3: controller-api database username
    --  $4: controller-api database password
    if not exists (
        select null from pg_user_mappings
            where srvname = 'controller_api_server' and usename = (select current_user)
    ) then
        create user mapping for current_user
            server controller_api_server options (user $3, password $4);
    end if;

    -- Import foreign schema from controller api if not exists
    if not exists (
        select null from information_schema.schemata
            where schema_name = 'controller_api'
    ) then
        create schema controller_api;
        import foreign schema public
            from server controller_api_server
            into controller_api;
    end if;

    -- =================================================
    -- End FDW definition
    -- =================================================

    -- Create V2 Schema
    create schema if not exists v2;
    
    -- Create V2 Tables and Views
    if not exists (
        select null from information_schema.views
            where table_name = 'deployments'
            and table_schema = 'v2'
    ) then
        create view v2.deployments as
            select controller_results.appid as appid,
                spacesapps.appname as name,
                spacesapps.space as space,
                spacesapps.plan as plan,
                spacesapps.instances as instances,
                spacesapps.healthcheck as healthcheck,
                spacesapps.port as port
            from spacesapps
            join (
                select apps.app as appid,
                       apps.name as appname,
                       spaces.name as spacename
                from controller_api.apps as apps
                join controller_api.spaces as spaces on apps.space = spaces.space
                where apps.deleted = false
            ) as controller_results on spacesapps.appname = controller_results.appname 
                and spacesapps.space = controller_results.spacename;
    end if;

    -- Create functions, triggers to handle inserts, updates, deletes, etc. on the view

    create function v2_deployments_insert_row() returns trigger as $v2_deployments_insert$
        begin
            -- Ignore the app ID and insert all other values into the spacesapps table
            insert into spacesapps(appname, space, plan, instances, healthcheck, port) 
                values(
                    NEW.name,
                    NEW.space,
                    NEW.plan,
                    NEW.instances,
                    NEW.healthcheck,
                    NEW.port
                );
            return NEW;
        end;
    $v2_deployments_insert$ language plpgsql;

    create function v2_deployments_delete_row() returns trigger as $v2_deployments_delete$
        begin
            -- Delete row from the spacesapps table
            delete from spacesapps where spacesapps.appname = OLD.name and spacesapps.space = OLD.space;
            return OLD;
        end;
    $v2_deployments_delete$ language plpgsql;

    create function v2_deployments_update_row() returns trigger as $v2_deployments_update$
        begin
            -- Update row in the spacesapps table, ignoring app ID
            update spacesapps set 
                appname = NEW.name,
                space = NEW.space,
                plan = NEW.plan,
                instances = NEW.instances,
                healthcheck = NEW.healthcheck,
                port = NEW.port
            where spacesapps.appname = OLD.name and spacesapps.space = OLD.space;
            return NEW;
        end;
    $v2_deployments_update$ language plpgsql;

    -- Override inserting rows on v2.deployments
    create trigger v2_deployments_insert
        instead of insert on v2.deployments
        for each row
        execute procedure v2_deployments_insert_row();

    -- Override deleting rows on v2.deployments
    create trigger v2_deployments_delete
        instead of delete on v2.deployments
        for each row
        execute procedure v2_deployments_delete_row();

    -- Override updating rows on v2.deployments
    create trigger v2_deployments_update
        instead of update on v2.deployments
        for each row
        execute procedure v2_deployments_update_row();

end
$$;
