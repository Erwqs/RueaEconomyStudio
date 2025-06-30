package eruntime

func (s *state) ClampResource() {
	for _, territory := range s.territories {
		// Clamp resources to their maximum storage capacity if they exceed it
		if territory == nil {
			continue
		}
		switch {
		case territory.Storage.At.Emeralds > territory.Storage.Capacity.Emeralds:
			territory.Storage.At.Emeralds = territory.Storage.Capacity.Emeralds
		case territory.Storage.At.Ores > territory.Storage.Capacity.Ores:
			territory.Storage.At.Ores = territory.Storage.Capacity.Ores
		case territory.Storage.At.Crops > territory.Storage.Capacity.Crops:
			territory.Storage.At.Crops = territory.Storage.Capacity.Crops
		case territory.Storage.At.Wood > territory.Storage.Capacity.Wood:
			territory.Storage.At.Wood = territory.Storage.Capacity.Wood
		case territory.Storage.At.Fish > territory.Storage.Capacity.Fish:
			territory.Storage.At.Fish = territory.Storage.Capacity.Fish
		}
	}
}
