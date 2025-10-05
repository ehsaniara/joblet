class Rnx < Formula
  desc "Cross-platform CLI for Joblet distributed job execution system"
  homepage "https://github.com/ehsaniara/joblet"
  license "MIT"

  # Dynamically fetch latest release info
  def self.get_latest_release
    require 'net/http'
    require 'json'
    require 'uri'

    uri = URI('https://api.github.com/repos/ehsaniara/joblet/releases/latest')

    begin
      response = Net::HTTP.get_response(uri)

      if response.code == '200'
        JSON.parse(response.body)
      else
        # Try to get list of releases as fallback
        fallback_uri = URI('https://api.github.com/repos/ehsaniara/joblet/releases')
        fallback_response = Net::HTTP.get_response(fallback_uri)

        if fallback_response.code == '200'
          releases = JSON.parse(fallback_response.body)
          # Get first non-prerelease or just first release
          stable = releases.find { |r| !r['prerelease'] } || releases.first

          if stable
            opoo "Using release #{stable['tag_name']} (unable to fetch 'latest' tag)"
            stable
          else
            odie "Unable to fetch release information from GitHub. Please check your internet connection and try again."
          end
        else
          odie "Unable to connect to GitHub API (HTTP #{response.code}). Please check your internet connection and try again."
        end
      end
    rescue StandardError => e
      odie "Failed to fetch latest release from GitHub: #{e.message}\n\nPlease check your internet connection or try again later."
    end
  end

  latest_release = get_latest_release
  latest_version = latest_release['tag_name']

  version latest_version.sub(/^v/, '')

  # Dynamic URLs based on latest release
  arch_suffix = Hardware::CPU.intel? ? 'amd64' : 'arm64'
  url "https://github.com/ehsaniara/joblet/releases/download/#{latest_version}/rnx-#{latest_version}-darwin-#{arch_suffix}-cli.tar.gz"

  def install
    # Install rnx binary
    bin.install "rnx-darwin-#{Hardware::CPU.intel? ? 'amd64' : 'arm64'}" => "rnx"

    # Generate shell completions (with error handling)
    begin
      output = Utils.safe_popen_read(bin / "rnx", "completion", "bash")
      (bash_completion / "rnx").write output if output && !output.empty?
    rescue => e
      opoo "Could not generate bash completions: #{e.message}"
    end

    begin
      output = Utils.safe_popen_read(bin / "rnx", "completion", "zsh")
      (zsh_completion / "_rnx").write output if output && !output.empty?
    rescue => e
      opoo "Could not generate zsh completions: #{e.message}"
    end

    begin
      output = Utils.safe_popen_read(bin / "rnx", "completion", "fish")
      (fish_completion / "rnx.fish").write output if output && !output.empty?
    rescue => e
      opoo "Could not generate fish completions: #{e.message}"
    end
  end

  def test
    assert_match version.to_s, shell_output("#{bin}/rnx --version")
  end

  def caveats
    <<~EOS
      📱 RNX CLI installed successfully!

      📋 Usage: rnx --help

      🔧 Configuration:
        Copy your rnx-config.yml to: ~/.rnx/
        Example: scp user@joblet-server:/opt/joblet/config/rnx-config.yml ~/.rnx/

      🎨 Admin UI (Separate Repository):
        The Admin UI is now available as a separate project:
        https://github.com/ehsaniara/joblet-admin

        To install and run the Admin UI:
          git clone https://github.com/ehsaniara/joblet-admin
          cd joblet-admin
          npm install
          npm run dev

      💡 The web admin UI provides a visual interface for managing jobs,
         viewing logs, and monitoring your Joblet servers.
    EOS
  end
end