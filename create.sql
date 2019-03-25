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
            VALUES ('scout', '256Mi', '256Mi', 10, 'Scout - 256MB RAM, $10/mo');
        INSERT INTO public.plans (name, memrequest, memlimit, price, "description") 
            VALUES ('scout-prod', '256Mi', '256Mi', 15, 'Scout (Production) - 256MB RAM, $15/mo');
        INSERT INTO public.plans (name, memrequest, memlimit, price, "description") 
            VALUES ('constellation', '256Mi', '512Mi', 20, 'Constellation - 256MB RAM, $20/mo');
        INSERT INTO public.plans (name, memrequest, memlimit, price, "description") 
            VALUES ('constellation-prod', '512Mi', '512Mi', 25, 'Constellation (Production) - 256MB RAM, $25/mo');
        INSERT INTO public.plans (name, memrequest, memlimit, price, "description") 
            VALUES ('akira', '768Mi', '1024Mi', 30, 'Akira - 768MB RAM, $30/mo');
        INSERT INTO public.plans (name, memrequest, memlimit, price, "description") 
            VALUES ('akria-prod', '1024Mi', '1024Mi', 35, 'Akira (Production) - 768MB RAM, $30/mo');
        INSERT INTO public.plans (name, memrequest, memlimit, price, "description") 
            VALUES ('galaxy', '1024Mi', '1536Mi', 40, 'Galaxy - 1024MB RAM, $40/mo');
        INSERT INTO public.plans (name, memrequest, memlimit, price, "description") 
            VALUES ('galaxy-prod', '1536Mi', '1536Mi', 45, 'Galaxy (Production) - 1024MB RAM, $40/mo');
        INSERT INTO public.plans (name, memrequest, memlimit, price, "description") 
            VALUES ('sovereign', '1536Mi', '2048Mi', 50, 'Sovereign - 1536MB RAM, $50/mo');
        INSERT INTO public.plans (name, memrequest, memlimit, price, "description") 
            VALUES ('sovereign-prod', '2048Mi', '2048Mi', 'Sovereign (Production) - 1536MB RAM, $50/mo');
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

end
$$;
