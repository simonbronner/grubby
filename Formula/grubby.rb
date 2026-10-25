class Grubby < Formula
  desc "Terminal cockpit TUI for GitHub repositories, PRs, and workspaces"
  homepage "https://github.com/simonbronner/grubby"
  version "7"
  license "MIT"

  on_macos do
    if Hardware::CPU.arm?
      url "https://github.com/simonbronner/grubby/releases/download/v7/grubby_v7_darwin_arm64.tar.gz"
      sha256 "964d7152a0b1ed150dee4cbc93a47871f46536b05438c2c5b760b6a8cb40bbd6"
    else
      url "https://github.com/simonbronner/grubby/releases/download/v7/grubby_v7_darwin_amd64.tar.gz"
      sha256 "a476883e53621684c517a8c54be6dd30f004be8ac939f68a6d1a335785634f41"
    end
  end

  on_linux do
    if Hardware::CPU.arm?
      url "https://github.com/simonbronner/grubby/releases/download/v7/grubby_v7_linux_arm64.tar.gz"
      sha256 "ad6cff40c5a52d192d9c5458687e01228d9e28cb2c2f796f8e8fd18a736038a1"
    else
      url "https://github.com/simonbronner/grubby/releases/download/v7/grubby_v7_linux_amd64.tar.gz"
      sha256 "41e11556cc231abd09d19da886ae13219fda4dfcef7cb91c498fc607ee3ef412"
    end
  end

  def install
    bin.install "grubby"
  end

  test do
    assert_match "grubby", shell_output("#{bin}/grubby -v")
  end
end
