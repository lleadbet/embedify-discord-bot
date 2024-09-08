# Embedify Discord Bot

## What does this bot do? 

There are too many damn services now blocking embeds, and so it's a quick bot to fix them with publicly available embed proxies. 

## How do I add?
To add to your channel: [https://discord.com/oauth2/authorize?client_id=1197310892866539671&permissions=274877933632&integration_type=0&scope=bot](https://discord.com/oauth2/authorize?client_id=1197310892866539671&permissions=274877933632&integration_type=0&scope=bot).

## Self-hosting

If you'd like to self-host, simply provide a `DISCORD_TOKEN` in a `.env` file (sample included). Review the Discord documentation for information on how to get started with building a Discord Bot. Otherwise, feel free to use the self-hosted version. 

## TikTok Embed Suppression

By default, the bot will suppress embeds on any messages that contain TikTok links; this is due to TikTok's terrible embed preview, and without any good way to remove specific embeds, it needs to remove all of them. 

You can disable this by changing the permissions to no longer "Manage Messages" via the installation URL, or if self-hosting, setting the `SUPPRESS_EMBEDS` environment variable to `false`. 

## Development

Feel free to submit PRs if you like, such as if you find that a given embed stops working for one reason or another, or you'd like to add another domain. 
