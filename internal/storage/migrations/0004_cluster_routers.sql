-- +migrate Up

-- +migrate StatementBegin
DELETE FROM lbrynet_servers;
INSERT INTO lbrynet_servers(name, address)
    VALUES
        ('default',  'http://lbrynet-0.sdk.default.svc.cluster.local:5279/'),
        ('lbrynet1', 'http://lbrynet-1.sdk.default.svc.cluster.local:5279/'),
        ('lbrynet2', 'http://lbrynet-2.sdk.default.svc.cluster.local:5279/'),
        ('lbrynet3', 'http://lbrynet-3.sdk.default.svc.cluster.local:5279/'),
        ('lbrynet4', 'http://lbrynet-4.sdk.default.svc.cluster.local:5279/');
        -- ('lbrynet5', 'http://lbrynet-5.sdk.default.svc.cluster.local:5279/');
        -- ('lbrynet6', 'http://lbrynet-6.sdk.default.svc.cluster.local:5279/');
        -- ('lbrynet7', 'http://lbrynet-7.sdk.default.svc.cluster.local:5279/');
        -- ('lbrynet8', 'http://lbrynet-8.sdk.default.svc.cluster.local:5279/');
        -- ('lbrynet9', 'http://lbrynet-9.sdk.default.svc.cluster.local:5279/');
        -- ('lbrynet10', 'http://lbrynet-10.sdk.default.svc.cluster.local:5279/');
-- +migrate StatementEnd

-- +migrate Down

-- +migrate StatementBegin
DELETE FROM lbrynet_servers;
INSERT INTO lbrynet_servers(name, address)
    VALUES
        ('default',  'http://lbrynet1:5279/'),
        ('lbrynet2', 'http://lbrynet2:5279/'),
        ('lbrynet3', 'http://lbrynet3:5279/'),
        ('lbrynet4', 'http://lbrynet4:5279/'),
        ('lbrynet5', 'http://lbrynet5:5279/');
-- +migrate StatementEnd
