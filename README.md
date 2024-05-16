Usage:

Install with `go install github.com/andre4ik3/update-dns`

```
Usage of update-dns:
  -domain string
        Domain to refresh (default: derived from value of -hostname)
  -hostname string
        Hostname to refresh (default: machine hostname)
  -proxy
        Whether the records should be proxied (default: false)
  -token string
        Cloudflare API token (can also be passed via CLOUDFLARE_API_TOKEN variable)
```

If your machine hostname is `foo.example.com` and you have a zone called
`example.com`, you're good to go (only token needed). But:

- If your machine hostname is different from the record name, use `-hostname`
- If your zone is a subdomain, use `-domain` to set it to the subdomain

To run automatically, put something like this into your crontab:

```
# Update dynamic DNS every hour
0   *   *   *   *   /path/to/update-dns -token="cloudflare-token"
```

FAQ:

- **Can this be done via a shell script?** Yes
- **Should this be done via a shell script?** Probably
- **Is it simpler to do it via a shell script?** Yes
- **So then why make this?** Idk
- **...**
