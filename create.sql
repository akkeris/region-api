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

    if not exists (SELECT NULL 
              FROM INFORMATION_SCHEMA.COLUMNS
             WHERE table_name = 'routerpaths'
              AND column_name = 'maintenance'
              and table_schema = 'public') then
        alter table routerpaths add column maintenance boolean default false not null;
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
            VALUES ('gp1', '256Mi', '256Mi', 10, '256MB RAM, 3.1 Intel Xeon Platinum 8000 CPU, 10 Gbps Networking');
        INSERT INTO public.plans (name, memrequest, memlimit, price, "description") 
            VALUES ('gp1-prod', '256Mi', '256Mi', 15, '256MB RAM, 3.1 Intel Xeon Platinum 8000 CPU, 10 Gbps Networking');
        INSERT INTO public.plans (name, memrequest, memlimit, price, "description") 
            VALUES ('gp2', '256Mi', '512Mi', 20, '512MB RAM, 3.1 Intel Xeon Platinum 8000 CPU, 10 Gbps Networking');
        INSERT INTO public.plans (name, memrequest, memlimit, price, "description") 
            VALUES ('gp2-prod', '512Mi', '512Mi', 25, '512MB RAM, 3.1 Intel Xeon Platinum 8000 CPU, 10 Gbps Networking');
        INSERT INTO public.plans (name, memrequest, memlimit, price, "description") 
            VALUES ('gp3', '768Mi', '1024Mi', 30, '1024MB RAM, 3.1 Intel Xeon Platinum 8000 CPU, 10 Gbps Networking');
        INSERT INTO public.plans (name, memrequest, memlimit, price, "description") 
            VALUES ('gp3-prod', '1024Mi', '1024Mi', 35, '1024MB RAM, 3.1 Intel Xeon Platinum 8000 CPU, 10 Gbps Networking');
        INSERT INTO public.plans (name, memrequest, memlimit, price, "description") 
            VALUES ('gp4', '1024Mi', '1536Mi', 40, '1536MB RAM, 3.1 Intel Xeon Platinum 8000 CPU, 10 Gbps Networking');
        INSERT INTO public.plans (name, memrequest, memlimit, price, "description") 
            VALUES ('gp4-prod', '1536Mi', '1536Mi', 45, '1536MB RAM, 3.1 Intel Xeon Platinum 8000 CPU, 10 Gbps Networking');
        INSERT INTO public.plans (name, memrequest, memlimit, price, "description") 
            VALUES ('mp1', '1536Mi', '2048Mi', 50, '2048MB RAM, 3.1 Intel Xeon Platinum 8000 CPU, 10 Gbps Networking');
        INSERT INTO public.plans (name, memrequest, memlimit, price, "description") 
            VALUES ('mp1-prod', '2048Mi', '2048Mi', 55, '2048MB RAM, 3.1 Intel Xeon Platinum 8000 CPU, 10 Gbps Networking');
        INSERT INTO public.plans (name, memrequest, memlimit, price, "description") 
            VALUES ('mp2', '3072Mi', '4096Mi', 55, '4096MB RAM, 3.1 Intel Xeon Platinum 8000 CPU, 10 Gbps Networking');
        INSERT INTO public.plans (name, memrequest, memlimit, price, "description") 
            VALUES ('mp2-prod', '4096Mi', '4096Mi', 55, '4096MB RAM, 3.1 Intel Xeon Platinum 8000 CPU, 10 Gbps Networking');    
    end if;

    if (select count(*) from plans where name = 'gp1' and description is null) > 0 then
        update plans set description = '256MB RAM, 3.1 Intel Xeon Platinum 8000 CPU, 10 Gbps Networking' where description is null and name = 'gp1';
    end if;

    if (select count(*) from plans where name = 'gp1-prod' and description is null) > 0 then
        update plans set description = '256MB RAM, 3.1 Intel Xeon Platinum 8000 CPU, 10 Gbps Networking' where description is null and name = 'gp1-prod';
    end if;

    if (select count(*) from plans where name = 'gp2' and description is null) > 0 then
        update plans set description = '512MB RAM, 3.1 Intel Xeon Platinum 8000 CPU, 10 Gbps Networking' where description is null and name = 'gp2';
    end if;

    if (select count(*) from plans where name = 'gp2-prod' and description is null) > 0 then
        update plans set description = '512MB RAM, 3.1 Intel Xeon Platinum 8000 CPU, 10 Gbps Networking' where description is null and name = 'gp2-prod';
    end if;

    if (select count(*) from plans where name = 'gp3' and description is null) > 0 then
        update plans set description = '1024MB RAM, 3.1 Intel Xeon Platinum 8000 CPU, 10 Gbps Networking' where description is null and name = 'gp3';
    end if;

    if (select count(*) from plans where name = 'gp3-prod' and description is null) > 0 then
        update plans set description = '1024MB RAM, 3.1 Intel Xeon Platinum 8000 CPU, 10 Gbps Networking' where description is null and name = 'gp3-prod';
    end if;

    if (select count(*) from plans where name = 'gp4' and description is null) > 0 then
        update plans set description = '1536MB RAM, 3.1 Intel Xeon Platinum 8000 CPU, 10 Gbps Networking' where description is null and name = 'gp4';
    end if;

    if (select count(*) from plans where name = 'gp4-prod' and description is null) > 0 then
        update plans set description = '1536MB RAM, 3.1 Intel Xeon Platinum 8000 CPU, 10 Gbps Networking' where description is null and name = 'gp4-prod';
    end if;

    if (select count(*) from plans where name = 'mp1' and description is null) > 0 then
        update plans set description = '2048MB RAM, 3.1 Intel Xeon Platinum 8000 CPU, 10 Gbps Networking' where description is null and name = 'mp1';
    end if;

    if (select count(*) from plans where name = 'mp1-prod' and description is null) > 0 then
        update plans set description = '2048MB RAM, 3.1 Intel Xeon Platinum 8000 CPU, 10 Gbps Networking' where description is null and name = 'mp1-prod';
    end if;

    if (select count(*) from plans where name = 'mp2' and description is null) > 0 then
        update plans set description = '4096MB RAM, 3.1 Intel Xeon Platinum 8000 CPU, 10 Gbps Networking' where description is null and name = 'mp2';
    end if;

    if (select count(*) from plans where name = 'mp2-prod' and description is null) > 0 then
        update plans set description = '4096MB RAM, 3.1 Intel Xeon Platinum 8000 CPU, 10 Gbps Networking' where description is null and name = 'mp2-prod';
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
end
$$;
