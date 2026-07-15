resource "synology_core_share_nfs_privilege" "media" {
  share_name = "media"

  rules {
    client      = "10.1.0.0/24"
    privilege   = "rw"
    root_squash = "root"
    async       = true
    crossmnt    = true
    insecure    = true

    security_flavor {
      sys = true
    }
  }

  rules {
    client      = "10.1.0.42"
    privilege   = "ro"
    root_squash = "root"

    security_flavor {
      kerberos_integrity = true
    }
  }
}
