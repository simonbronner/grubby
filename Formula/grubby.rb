class Grubby < Formula
  desc "Terminal cockpit TUI for GitHub repositories, PRs, and workspaces"
  homepage "https://github.com/simonbronner/grubby"
  version "6"
  license "MIT"

  on_macos do
    if Hardware::CPU.arm?
      url "https://github.com/simonbronner/grubby/releases/download/v6/grubby_v6_darwin_arm64.tar.gz"
      sha256 "aee7bb560ff8e0b36cb55644ec364ee3240678dfee5d611d7a42a53976366161"
    else
      url "https://github.com/simonbronner/grubby/releases/download/v6/grubby_v6_darwin_amd64.tar.gz"
      sha256 "bfe9121320dbd88102d0b9a4df46e26cd8bb80ede9bdfb44c9f6a6ce98b3c430"
    end
  end

  on_linux do
    if Hardware::CPU.arm?
      url "https://github.com/simonbronner/grubby/releases/download/v6/grubby_v6_linux_arm64.tar.gz"
      sha256 "bdb59416399e24e14a62411fd18326603566f93c901a644cc1f9d8eaf636f043"
    else
      url "https://github.com/simonbronner/grubby/releases/download/v6/grubby_v6_linux_amd64.tar.gz"
      sha256 "a4b3431afa6f517eab987a7aeec5ea7cf8ab73403c82629ea34c23521da6dbc4"
    end
  end

  def install
    bin.install "grubby"
  end

  test do
    assert_match "grubby", shell_output("#{bin}/grubby -v")
  end
end
