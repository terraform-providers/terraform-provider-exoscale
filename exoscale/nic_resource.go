package exoscale

import (
	"fmt"
	"net"

	"github.com/exoscale/egoscale"
	"github.com/hashicorp/terraform/helper/schema"
)

func nicResource() *schema.Resource {
	return &schema.Resource{
		Create: createNic,
		Exists: existsNic,
		Read:   readNic,
		Delete: deleteNic,

		Schema: map[string]*schema.Schema{
			"compute_id": {
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
			},
			"network_id": {
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
			},
			"ip_address": {
				Type:         schema.TypeString,
				Optional:     true,
				ForceNew:     true,
				Description:  "IP address",
				ValidateFunc: ValidateIPv4String,
			},
			"netmask": {
				Type:     schema.TypeString,
				Computed: true,
			},
			"gateway": {
				Type:     schema.TypeString,
				Computed: true,
			},
			"mac_address": {
				Type:     schema.TypeString,
				Computed: true,
			},
			// XXX add the IPv6 fields
		},
	}
}

func createNic(d *schema.ResourceData, meta interface{}) error {
	client := GetComputeClient(meta)
	async := meta.(BaseConfig).async

	var ip net.IP
	if i, ok := d.GetOk("ip_address"); ok {
		ip = net.ParseIP(i.(string))
	}

	networkID := d.Get("network_id").(string)

	resp, err := client.AsyncRequest(&egoscale.AddNicToVirtualMachine{
		NetworkID:        networkID,
		VirtualMachineID: d.Get("compute_id").(string),
		IPAddress:        ip,
	}, async)

	if err != nil {
		return err
	}

	vm := resp.(*egoscale.AddNicToVirtualMachineResponse).VirtualMachine
	nic := vm.NicByNetworkID(networkID)
	if nic != nil {
		d.SetId(nic.ID)
		return readNic(d, meta)
	} else {
		return fmt.Errorf("Nic addition didn't create a NIC for Network %s", networkID)
	}
}

func readNic(d *schema.ResourceData, meta interface{}) error {
	client := GetComputeClient(meta)
	resp, err := client.Request(&egoscale.ListNics{
		NicID:            d.Id(),
		VirtualMachineID: d.Get("compute_id").(string),
	})

	if err != nil {
		return handleNotFound(d, err)
	}

	nics := resp.(*egoscale.ListNicsResponse)
	if nics.Count == 0 {
		return fmt.Errorf("No nic found for ID: %s", d.Id())
	}

	nic := nics.Nic[0]
	return applyNic(d, nic)
}

func existsNic(d *schema.ResourceData, meta interface{}) (bool, error) {
	client := GetComputeClient(meta)
	resp, err := client.Request(&egoscale.ListNics{
		NicID:            d.Id(),
		VirtualMachineID: d.Get("compute_id").(string),
	})

	if err != nil {
		e := handleNotFound(d, err)
		return d.Id() != "", e
	}

	nics := resp.(*egoscale.ListNicsResponse)
	if nics.Count == 0 {
		d.SetId("")
		return false, nil
	}

	return true, nil
}

func deleteNic(d *schema.ResourceData, meta interface{}) error {
	client := GetComputeClient(meta)
	async := meta.(BaseConfig).async

	resp, err := client.AsyncRequest(&egoscale.RemoveNicFromVirtualMachine{
		NicID:            d.Id(),
		VirtualMachineID: d.Get("compute_id").(string),
	}, async)

	if err != nil {
		return err
	}

	vm := resp.(*egoscale.RemoveNicFromVirtualMachineResponse).VirtualMachine
	nic := vm.NicByNetworkID(d.Get("network_id").(string))
	if nic != nil {
		return fmt.Errorf("Failed removing NIC %s from instance %s", d.Id(), vm.ID)
	}

	d.SetId("")
	return nil
}

func applyNic(d *schema.ResourceData, nic egoscale.Nic) error {
	d.SetId(nic.ID)
	d.Set("compute_id", nic.VirtualMachineID)
	d.Set("network_id", nic.NetworkID)
	d.Set("ip_address", nic.IPAddress.String())
	d.Set("netmask", nic.Netmask.String())
	d.Set("gateway", nic.Gateway.String())
	d.Set("mac_address", nic.MacAddress)

	return nil
}
