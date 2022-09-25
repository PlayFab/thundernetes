---
layout: default
title: Running docs locally
parent: Development
nav_order: 3
---

# Running docs locally

We use [GitHub Pages](https://docs.github.com/en/pages) to host Thundernetes documentation. To preview your changes locally:

- [WSL](https://docs.microsoft.com/en-us/windows/wsl/install) is recommended
- Follow the instructions [here](https://docs.github.com/en/pages/setting-up-a-github-pages-site-with-jekyll/testing-your-github-pages-site-locally-with-jekyll). Specifically, install `Ruby` and `Bundler`. `Jekyll` needs to be installed as well but it's already in the Gemfile so it should be installed automatically.
- Switch to the `docs` directory
- Run `bundle install` to install the necessary prerequisites
- Run `bundle exec jekyll serve --config _config-development.yml`
- Browse the site on http://localhost:4000/thundernetes

Alternatively, you can use [this container image](https://github.com/BretFisher/jekyll-serve) with a command similar to the following, once you are in the `docs` directory:

{% include code-block-start.md %}
docker run -p 4000:4000 --env JEKYLL_ENV=production --rm -v $(pwd):/site bretfisher/jekyll-serve bundle exec jekyll serve --force-polling --config _config-development.yml --host 0.0.0.0
{% include code-block-end.md %}
