## Contributing

Contributions are always welcome!

### Developer Certificate of Origin (DCO)

This project uses the [Developer Certificate of Origin (DCO)](https://developercertificate.org/) to confirm that contributors are legally authorized to make their contributions. You must sign off on each commit by adding a `Signed-off-by` line.

**How to sign off**

- **When committing:** Use `git commit -s` so Git adds the line automatically using your configured name and email.
- **Or add manually:** The last line of your commit message must be:
  ```
  Signed-off-by: Your Name <your.email@example.com>
  ```

By signing off, you certify that you have the right to submit the work under the project’s license (see [LICENSE](LICENSE)).

- **Code style:** Follow the [Google Go Style Guide](https://google.github.io/styleguide/go/).
- **Before submitting:** Run `make test` and `make vet` (or `make lint`) and fix any issues.
