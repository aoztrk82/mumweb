"""Update the Docker Hub repository description and full description (README).

Uses the credentials Docker Desktop already holds for docker.io. Nothing secret is printed.
Usage: python scripts/dockerhub-description.py docs/DOCKERHUB.md  (or: make hub-readme)
"""
import json, subprocess, sys, urllib.request

REPO = 'aoztrk82/mumweb'
SHORT = 'Self-hosted web client for Mumble voice chat: talk from your browser, no install. amd64 + arm64.'
assert len(SHORT) <= 100, len(SHORT)

full = open(sys.argv[1], encoding='utf-8').read().replace('\r\n', '\n')
assert len(full) < 25000

out = subprocess.run(['docker-credential-desktop', 'get'], input='https://index.docker.io/v1/',
                     capture_output=True, text=True, check=True).stdout
cred = json.loads(out)
user, secret = cred['Username'], cred['Secret']
print('credentials for', user, '(secret length %d, not shown)' % len(secret))

def call(url, body, method='POST', token=None):
    req = urllib.request.Request(url, data=json.dumps(body).encode(), method=method,
                                 headers={'Content-Type': 'application/json'})
    if token:
        req.add_header('Authorization', 'JWT ' + token)
    with urllib.request.urlopen(req, timeout=30) as r:
        return json.load(r)

login = call('https://hub.docker.com/v2/users/login', {'username': user, 'password': secret})
token = login['token']
res = call(f'https://hub.docker.com/v2/repositories/{REPO}/', {'description': SHORT, 'full_description': full},
           method='PATCH', token=token)
print('description:', res.get('description'))
print('full_description length:', len(res.get('full_description') or ''))
