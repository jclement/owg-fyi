I want a great gemini site/presence.

I'm going to host it at owg.fyi (DNS already setup and you can SSH to root)
host is empty.  setup docker, setup docker compose stack.  make sure images pull.

ideally...
supports content in markdown or gemtext or whatever

docker-compose stack
Ideally, builds a docker image which combines server, content.
binds to 80, 443, gemini port.
auto real SQL with let's encrypt

mise run dev - runs local live version for evaling content
self-signed cert for gemini

web should server shows same content with a clean, but simple, CSS to make it look nice.

Github Aciton to build container.
We'll use  github.com/jclement/pullpilot (in docker compose - for refreshes)


Probably golang based server that does all of this?
Ideally supports basic SSI type thing.  _header.md/.gemtext or whetever in folder.  In herited.  Same for footer.
Maybe also some syntax for including a directory listing?

Sexy colored logging where appropriate.
Self-signed certificate by default

Design this great.  Single user instance.  Maybe support taht Titan file upload thing too, if it's safe (client certificate?).  Review molly brown project for ideas relevant to this.

Steal content from ~/Developer/owg and ~/Developer/owg-blog (not all blog posts)
to build a site structure that makes sense for contact page.  List of cool projects.  Links.  Mini gemlog, etc.  Whatever would be a great start.

Can it have built-in site search.
