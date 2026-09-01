module github.com/Muxcore-Media/database-postgres

go 1.26.4

require (
	github.com/Muxcore-Media/core v0.5.8
	github.com/Muxcore-Media/core/pkg/contracts v0.5.8
	github.com/Muxcore-Media/core/sdk/go/module v0.5.8
	github.com/jackc/pgx/v5 v5.7.5
	google.golang.org/grpc v1.82.1
)

require (
	github.com/Muxcore-Media/contracts-media v0.1.0 // indirect
	github.com/jackc/pgpassfile v1.0.0 // indirect
	github.com/jackc/pgservicefile v0.0.0-20240606120523-5a60cdf6a761 // indirect
	github.com/jackc/puddle/v2 v2.2.2 // indirect
	golang.org/x/crypto v0.54.0 // indirect
	golang.org/x/net v0.56.0 // indirect
	golang.org/x/sync v0.22.0 // indirect
	golang.org/x/sys v0.47.0 // indirect
	golang.org/x/text v0.40.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20260610212136-7ab31c22f7ad // indirect
	google.golang.org/protobuf v1.36.11 // indirect
)

replace github.com/Muxcore-Media/contracts-media => /home/ender/Projects/MuxCore/contracts-media

replace github.com/Muxcore-Media/core/pkg/contracts => /home/ender/Projects/MuxCore/core/pkg/contracts

replace github.com/Muxcore-Media/core => /home/ender/Projects/MuxCore/core

replace github.com/Muxcore-Media/core/sdk/go/module => /home/ender/Projects/MuxCore/core/sdk/go/module

replace github.com/Muxcore-Media/contracts-notification => /home/ender/Projects/MuxCore/contracts-notification

replace github.com/Muxcore-Media/contracts-playback => /home/ender/Projects/MuxCore/contracts-playback

replace github.com/Muxcore-Media/contracts-scanner => /home/ender/Projects/MuxCore/contracts-scanner

replace github.com/Muxcore-Media/contracts-automation => /home/ender/Projects/MuxCore/contracts-automation

replace github.com/Muxcore-Media/contracts-metadata => /home/ender/Projects/MuxCore/contracts-metadata

replace github.com/Muxcore-Media/contracts-media-admin => /home/ender/Projects/MuxCore/contracts-media-admin

replace github.com/Muxcore-Media/contracts-downloader => /home/ender/Projects/MuxCore/contracts-downloader

replace github.com/Muxcore-Media/contracts-indexer => /home/ender/Projects/MuxCore/contracts-indexer

replace github.com/Muxcore-Media/core/pkg/tenant => /home/ender/Projects/MuxCore/core/pkg/tenant

replace github.com/Muxcore-Media/core/sdk/go/client => /home/ender/Projects/MuxCore/core/sdk/go/client
