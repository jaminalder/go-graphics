# ColorLisa palette dataset

Source: <https://colorlisa.com/> (fetched 2026-07-21). This is the master
color data for `internal/palette` — implement the Go palette table from this
file; do not re-scrape the site. 5 colors per palette, listed in site order.

Slug convention: `<artist-lastname>-<short-artwork-key>`, kebab-case, unique.
The slugs below are the canonical CLI/palette IDs.

Also keep (not from ColorLisa) the original staticart sketch_7 palette:

| slug | source | colors |
|---|---|---|
| `staticart-seven` | staticart sketch_7 | `#ED6A5A` `#F4F1BB` `#9BC1BC` `#5CA4A9` `#E6EBE0` |

## Dataset

| slug | artist — artwork | colors |
|---|---|---|
| `albers-luminous-day` | Josef Albers — Adobe (Variant): Luminous Day | `#D77186` `#61A2DA` `#6CB7DA` `#B5B5B3` `#D75725` |
| `albers-tehuana` | Josef Albers — Homage to the Square (La Tehuana) | `#C00559` `#DE1F6C` `#F3A20D` `#F07A13` `#DE6716` |
| `albrecht-golden-cloud` | Gretchen Albrecht — Golden Cloud | `#171635` `#00225D` `#763262` `#CA7508` `#E9A621` |
| `apple-rainbow` | Billy Apple — Rainbow | `#F24D98` `#813B7C` `#59D044` `#F3A002` `#F2F44D` |
| `arnoldi-spar` | Per Arnoldi — Spar | `#C2151B` `#2021A0` `#3547B3` `#E2C43F` `#E0DCDD` |
| `avery-bicycle-rider` | Milton Avery — Bicycle Rider By The Loire | `#F3C937` `#7B533E` `#BFA588` `#604847` `#552723` |
| `avery-cello-player` | Milton Avery — Cello Player | `#E2CACD` `#2E7CA8` `#F1C061` `#DA7338` `#741D13` |
| `afklint-swan` | Hilma af Klint — The Swan | `#D6CFC4` `#466CA6` `#D1AE45` `#87240E` `#040204` |
| `basquiat-black-king` | Jean-Michel Basquiat — Untitled (Black King Catch Scorpio) | `#8CABD9` `#F6A7B8` `#F1EC7A` `#1D4D9F` `#F08838` |
| `basquiat-dustheads` | Jean-Michel Basquiat — DUSTHEADS | `#C11432` `#009ADA` `#66A64F` `#FDD10A` `#070707` |
| `beckmann-dancing-bar` | Max Beckmann — Dancing Bar In Baden-Baden | `#4B3A51` `#A77A4B` `#ECC6A2` `#A43020` `#722D24` |
| `botero-seated-nude` | Fernando Botero — Seated Nude | `#99B6BD` `#B3A86A` `#ECC9A0` `#D4613E` `#BB9568` |
| `botticelli-venus` | Sandro Botticelli — The Birth Of Venus | `#7A989A` `#849271` `#C1AE8D` `#CF9546` `#C67052` |
| `botticelli-dante` | Sandro Botticelli — Portrait Of Dante | `#272725` `#DDBD85` `#DA694F` `#A54A48` `#FDFFE5` |
| `bruegel-icarus` | Pieter Bruegel — Landscape with the Fall of Icarus | `#BFBED5` `#7F9086` `#A29A68` `#676A4F` `#A63C24` |
| `bush-striped-column` | Jack Bush — Striped Column | `#529DCB` `#ECA063` `#71BF50` `#F3CC4F` `#D46934` |
| `bush-nude` | Jack Bush — Nude | `#A1D8B6` `#D2C48E` `#F45F40` `#F9AE8D` `#80B9CE` |
| `cassatt-letter` | Mary Cassatt — The Letter | `#1C5679` `#BBB592` `#CAC3B2` `#808C5C` `#5F4B3B` |
| `cezanne-bathers` | Paul Cézanne — The Bathers | `#8399B3` `#697A55` `#C4AA88` `#B68E52` `#8C5B28` |
| `chagall-mariee` | Marc Chagall — La Mariée sous le Baldaquin | `#3F6F76` `#69B7CE` `#C65840` `#F4CE4B` `#62496F` |
| `coolidge-poker-game` | C.M. Coolidge — Poker Game, 1894 | `#204035` `#4A7169` `#BEB59C` `#735231` `#49271B` |
| `dali-memory` | Salvador Dalí — The Persistence of Memory | `#40798C` `#BCA455` `#BFB37F` `#805730` `#514A2E` |
| `dali-apparition` | Salvador Dalí — Apparition of Face and Fruit Dish on a Beach | `#9BC0CC` `#CAD8D8` `#D0CE9F` `#806641` `#534832` |
| `davinci-mona-lisa` | Leonardo da Vinci — Mona Lisa | `#C8B272` `#A88B4C` `#A0A584` `#697153` `#43362A` |
| `davis-anthracite-minuet` | Gene Davis — Anthracite Minuet | `#293757` `#568D4B` `#D5BB56` `#D26A1B` `#A41D1A` |
| `dechirico-melancholy` | Giorgio de Chirico — Mystery and Melancholy of a Street | `#27403D` `#48725C` `#212412` `#F3E4C2` `#D88F2E` |
| `dechirico-red-tower` | Giorgio de Chirico — The Red Tower | `#2992BF` `#4CBED9` `#292C17` `#F9F6EF` `#F0742A` |
| `degas-milliner` | Edgar Degas — At the Milliner's | `#BDB592` `#ACBBC5` `#9E8D3D` `#8C4F36` `#2C2D2C` |
| `delaunay-bleriot` | Robert Delaunay — Hommage a Blériot | `#4368B6` `#78A153` `#DEC23B` `#E4930A` `#C53211` |
| `delaunay-air-iron-water` | Robert Delaunay — Air, Iron, and Water | `#A4B7E1` `#B8B87A` `#EFDE80` `#EFBD37` `#A85E5E` |
| `demuth-figure-5` | Charles Demuth — I Saw the Figure 5 in Gold | `#E4AF79` `#DF9C41` `#AF7231` `#923621` `#2D2A28` |
| `diebenkorn-seawall` | Richard Diebenkorn — Seawall | `#2677A5` `#639BC1` `#639BC1` `#90A74A` `#5D8722` |
| `dix-nun` | Otto Dix — The Nun | `#1E1D20` `#B66636` `#547A56` `#BDAE5B` `#515A7C` |
| `dix-mayer-hermann` | Otto Dix — Dr. Mayer-Hermann | `#E0DBC8` `#C9BE90` `#76684B` `#CDAB7E` `#3C2B23` |
| `duchamp-landscape` | Marcel Duchamp — Landscape | `#D0CEC2` `#7BAA80` `#4B6B5E` `#BF9A41` `#980019` |
| `durer-turf` | Albrecht Dürer — The Large Piece of Turf | `#657359` `#9AA582` `#8B775F` `#D7C9BE` `#F1E4DB` |
| `ernst-woman-old-man` | Max Ernst — Woman, Old Man, and Flower | `#91323A` `#3A4960` `#D7C969` `#6D7345` `#554540` |
| `escher-gravity` | M.C. Escher — Gravity | `#C1395E` `#AEC17B` `#F0CA50` `#E07B42` `#89A7C2` |
| `feeley-april-15` | Paul Feeley — Untitled (April 15) | `#2C458D` `#E4DFD9` `#425B4F` `#EBAD30` `#BF2124` |
| `feitelson-1964` | Lorser Feitelson — Untitled (1964) | `#202221` `#661E2A` `#AB381B` `#EAD4A3` `#E3DED8` |
| `frankenthaler-mauve-district` | Helen Frankenthaler — Mauve District | `#5D7342` `#D7AE04` `#ECD7B8` `#A58B8C` `#272727` |
| `freud-girl-kitten` | Lucian Freud — Girl with a Kitten | `#E1D2BD` `#A77E5E` `#2D291D` `#85868B` `#83774D` |
| `frost-suspended-forms` | Terry Frost — Suspended Forms | `#EF5950` `#8D5A78` `#C66F26` `#FB6B22` `#DC2227` |
| `gauguin-siesta` | Paul Gauguin — The Siesta | `#21344F` `#8AAD05` `#E2CE1B` `#DF5D22` `#E17976` |
| `geiger-pink-orange` | Rupprecht Geiger — Untitled (Pink and orange) | `#FF62A9` `#F77177` `#FA9849` `#FE6E3A` `#FD5A35` |
| `hockney-bigger-splash` | David Hockney — A Bigger Splash | `#79BED9` `#126599` `#CABD8A` `#497E58` `#B28D7B` |
| `hockney-man-shower` | David Hockney — Man in Shower in Beverly Hills | `#BF9FB7` `#A65899` `#55AC97` `#ECBF00` `#211F20` |
| `hockney-lawson-sleep` | David Hockney — George Lawson and Wayne Sleep | `#A63F5A` `#B4D2D9` `#3E797A` `#F2E7AE` `#8C7D61` |
| `hofmann-wind` | Hans Hofmann — The Wind | `#1A6DED` `#2C7CE6` `#145CBF` `#162B3D` `#F9ECE4` |
| `hokusai-great-wave` | Katsushika Hokusai — The Great Wave off Kanagawa | `#1F284C` `#2D4472` `#6E6352` `#D9CCAC` `#ECE2C6` |
| `homer-clams` | Winslow Homer — A Basket of Clams | `#A9944A` `#F2D9B3` `#725435` `#8E9DBF` `#BD483C` |
| `hopper-night-windows` | Edward Hopper — Night Windows | `#67161C` `#3F6148` `#DBD3A4` `#A4804C` `#4B5F80` |
| `indiana-love` | Robert Indiana — Love, Indiana Stable May 66 | `#2659D8` `#1C6FF3` `#5EBC4E` `#53A946` `#F24534` |
| `jean-rift-scull` | James Jean — RIFT SCULL | `#51394E` `#F6DE7D` `#C8AF8A` `#658385` `#B04838` |
| `johns-target` | Jasper Johns — Target | `#4B6892` `#F9E583` `#FED43F` `#F6BD28` `#BE4C46` |
| `kahlo-self-portrait` | Frida Kahlo — Self-Portrait | `#121510` `#6D8325` `#D6CFB7` `#E5AD4F` `#BD5630` |
| `kandinsky-apple-tree` | Wassily Kandinsky — Apple Tree | `#5D7388` `#A08F27` `#E5A729` `#4F4D1D` `#8AAE8A` |
| `kandinsky-soft-pressure` | Wassily Kandinsky — Soft Pressure | `#D2981A` `#A53E1F` `#457277` `#8DCEE2` `#8F657D` |
| `kandinsky-zig-zags` | Wassily Kandinsky — White Zig Zags | `#C13C53` `#DA73A8` `#4052BD` `#EFE96D` `#D85143` |
| `klee-destruction-hope` | Paul Klee — Destruction and Hope | `#A7B3CD` `#E6DA9E` `#676155` `#CDB296` `#CCD7AD` |
| `klee-fire-evening` | Paul Klee — Fire Evening | `#4F51FE` `#8C1E92` `#FF4E0B` `#CD2019` `#441C21` |
| `klein-anthropometry` | Yves Klein — Anthropometry: Princess Helena | `#344CB9` `#1B288A` `#0F185B` `#D7C99A` `#F2E4C7` |
| `klimt-kiss` | Gustav Klimt — The Kiss | `#4A5FAB` `#609F5C` `#E3C454` `#A27CBA` `#B85031` |
| `koons-pink-panther` | Jeff Koons — Pink Panther | `#D6AABE` `#B69F7F` `#ECD9AD` `#76A9A2` `#A26775` |
| `krasner-1949` | Lee Krasner — Untitled (1949) | `#333333` `#D1B817` `#2A2996` `#B34325` `#C8CCC6` |
| `lawrence-street-shadows` | Jacob Lawrence — Street Shadows | `#614671` `#BE994A` `#C8B595` `#BD4335` `#8B3834` |
| `lawrence-library` | Jacob Lawrence — The Library | `#5E3194` `#9870B9` `#F1B02F` `#EA454C` `#CC0115` |
| `lewitt-squiggles` | Sol LeWitt — Squiggles | `#0A71B6` `#F9C40A` `#190506` `#EB5432` `#EAF2F0` |
| `lichtenstein-kiss-ii` | Roy Lichtenstein — Kiss II | `#3229AD` `#BC000E` `#E7CFB7` `#FFEC04` `#090109` |
| `lichtenstein-hopeless` | Roy Lichtenstein — Hopeless | `#00020E` `#FFDE01` `#A5BAD6` `#F1C9C7` `#BD0304` |
| `lichtenstein-girl-ball` | Roy Lichtenstein — Girl with Ball | `#C7991F` `#C63D33` `#23254C` `#E0C4AE` `#D5D0B2` |
| `malevich-suprematist` | Kazimir Malevich — Suprematist Composition | `#151817` `#001A56` `#197C3F` `#D4A821` `#C74C25` |
| `manet-boating` | Édouard Manet — Boating | `#6486AD` `#2D345D` `#D9BE7F` `#5A3A26` `#C6A490` |
| `magritte-son-of-man` | René Magritte — The Son of Man | `#B60614` `#3A282F` `#909018` `#E3BFA1` `#EE833E` |
| `magritte-menaced-assassin` | René Magritte — The Menaced Assassin | `#B6B3BB` `#697D8E` `#B8B87E` `#6F5F4B` `#292A2D` |
| `masaccio-tax-collector` | Masaccio — Jesus, Saint Peter and the Tax Collector | `#0E2523` `#324028` `#C26B61` `#5A788D` `#DE7944` |
| `matisse-dance` | Henri Matisse — Dance (I) | `#2168A6` `#165F8C` `#03735E` `#078C66` `#F2A29B` |
| `matisse-collioure` | Henri Matisse — Landscape at Collioure | `#375C8C` `#A69649` `#BF7821` `#BFAA8F` `#D95E52` |
| `michelangelo-adam` | Michelangelo — The Creation of Adam | `#42819F` `#86AA7D` `#CBB396` `#555234` `#4D280F` |
| `miro-woman-dog-moon` | Joan Miró — Woman and Dog in Front of the Moon | `#C04759` `#3B6C73` `#383431` `#F1D87F` `#EDE5D2` |
| `modigliani-zborowska` | Amedeo Modigliani — Anna Zborowska | `#1D2025` `#45312A` `#7E2F28` `#202938` `#D58E40` |
| `mondrian-boogie-woogie` | Piet Mondrian — Broadway Boogie Woogie | `#314290` `#4A71C0` `#F1F2ED` `#F0D32D` `#AB3A2C` |
| `monet-sunflowers` | Claude Monet — Bouquet of Sunflowers | `#184430` `#548150` `#DEB738` `#734321` `#852419` |
| `monet-water-lilies` | Claude Monet — Water Lilies (1906) | `#9F4640` `#4885A4` `#395A92` `#7EA860` `#B985BA` |
| `monet-parasol` | Claude Monet — Woman with a Parasol | `#82A4BC` `#4C7899` `#2F5136` `#B1B94C` `#E5DCBE` |
| `munch-scream-pastel` | Edvard Munch — The Scream (pastel) | `#5059A1` `#EFC337` `#1F386E` `#D1AE82` `#BE3B2C` |
| `munch-scream-oil` | Edvard Munch — The Scream (oil) | `#272A2A` `#E69253` `#EDB931` `#E4502E` `#4378A0` |
| `newman-vir-heroicus` | Barnett Newman — Vir Heroicus Sublimis | `#442327` `#C0BC98` `#A6885D` `#8A3230` `#973B2B` |
| `noland-turnsole` | Kenneth Noland — Turnsole | `#D0D8CD` `#586180` `#E2AC29` `#1A1915` `#E6E1CE` |
| `okeeffe-abstraction-blue` | Georgia O'Keeffe — Abstraction Blue | `#0E122D` `#182044` `#51628E` `#91A1BA` `#AFD0C9` |
| `oldenburg-red-tights` | Claes Oldenburg — Red Tights with Fragment 9 | `#95B1C9` `#263656` `#698946` `#F8D440` `#C82720` |
| `picasso-demoiselles` | Pablo Picasso — Les Demoiselles d'Avignon | `#CD6C74` `#566C7D` `#DD9D91` `#A1544B` `#D5898D` |
| `picasso-dream` | Pablo Picasso — The Dream | `#4E7989` `#A9011B` `#E4A826` `#80944E` `#DCD6B2` |
| `pollock-number-1` | Jackson Pollock — Number 1 | `#D89CA9` `#1962A0` `#F1ECD7` `#E8C051` `#1A1C23` |
| `prince-purple-rain` | Prince — Purple Rain | `#735BCC` `#6650B4` `#59449C` `#4B3984` `#3E2D6C` |
| `quidor-leatherstocking` | John Quidor — Leatherstocking's Rescue | `#B79A59` `#826C37` `#54442F` `#DBCEAF` `#C4AA52` |
| `ramos-tobacco-rose` | Mel Ramos — Tobacco Rose | `#C13E43` `#376EA5` `#565654` `#F9D502` `#E7CA6B` |
| `redon-green-vase` | Odilon Redon — Large Green Vase with Mixed Flowers | `#695B8F` `#B26C61` `#C2AF46` `#4D5E30` `#8B1F1D` |
| `rembrandt-night-watch` | Rembrandt — The Night Watch | `#DBC99A` `#A68329` `#5B5224` `#8A350C` `#090A04` |
| `renoir-moulin-galette` | Pierre-Auguste Renoir — Dance at Le moulin de la Galette | `#2B5275` `#A69F55` `#F1D395` `#FFFBDD` `#D16647` |
| `renoir-piano` | Pierre-Auguste Renoir — Young Girls at the Piano | `#303241` `#B7A067` `#C8C2B2` `#686D4F` `#4D3930` |
| `riley-diver` | Bridget Riley — Diver | `#FAB9AC` `#7BBC53` `#DE6736` `#67C1EC` `#E6B90D` |
| `rosenquist-marilyn` | James Rosenquist — Marilyn Monroe, I | `#E25D75` `#3F4C8C` `#6A79B0` `#D7BC1F` `#DCCFAB` |
| `rothko-1968` | Mark Rothko — Untitled (1968) | `#E49A16` `#E19713` `#D67629` `#DA6E2E` `#D85434` |
| `rothko-white-black-rust` | Mark Rothko — Untitled (White, Black, Rust, on Brown) | `#D5D6D1` `#BEC0BF` `#5B382C` `#39352C` `#1B1B1B` |
| `sargent-vickers-children` | John Singer Sargent — Garden Study of the Vickers Children | `#B43A35` `#3E501E` `#F8F2F4` `#6B381D` `#20242D` |
| `sargent-villa-marlia` | John Singer Sargent — Villa di Marlia, Lucca | `#778BD0` `#E2D76B` `#95BF78` `#4E6A3D` `#5F4F38` |
| `sargent-carnation-lily` | John Singer Sargent — Carnation, Lily, Lily, Rose | `#EEC7A0` `#EAA69C` `#BD7C96` `#A1A481` `#D97669` |
| `schlemmer-bauhaus-stairway` | Oskar Schlemmer — Bauhaus Stairway | `#3A488A` `#8785B2` `#DABD61` `#D95F30` `#BE3428` |
| `seurat-grande-jatte` | Georges Seurat — A Sunday on La Grande Jatte | `#3F3F63` `#808EB7` `#465946` `#8C9355` `#925E48` |
| `skoglund-radioactive-cats` | Sandy Skoglund — RADIOACTIVE CATS | `#D7F96E` `#457D24` `#879387` `#E1C39F` `#394835` |
| `tchelitchew-hide-and-seek` | Pavel Tchelitchew — Hide-and-Seek | `#AC2527` `#F8CC5A` `#5C8447` `#61221A` `#2B4868` |
| `turner-val-daosta` | J. M. W. Turner — A mountain scene, Val d'Aosta | `#F1ECCE` `#9EA3B5` `#E9D688` `#A85835` `#AE8045` |
| `twombly-1961` | Cy Twombly — Untitled, 1961 | `#F2788F` `#F591EA` `#F0C333` `#F5C2AF` `#F23B3F` |
| `ulrich-wine-barrels` | Johann Jacob Ulrich — Wine Barrels Sailing Barge | `#FDDDAB` `#E7A974` `#A87250` `#7B533D` `#6A4531` |
| `vandoesburg-russian-dance` | Theo van Doesburg — Rhythm of a Russian Dance | `#BD748F` `#3D578E` `#BFAB68` `#DAD7D0` `#272928` |
| `vandoesburg-composition` | Theo van Doesburg — Study for a Composition | `#53628D` `#B8B45B` `#C1C3B6` `#984F48` `#2E3432` |
| `vaneyck-arnolfini` | Jan van Eyck — The Arnolfini Portrait | `#3C490C` `#3B5B71` `#262121` `#7C6C4E` `#6C2B23` |
| `vangogh-starry-night` | Vincent van Gogh — The Starry Night | `#1A3431` `#2B41A7` `#6283C8` `#CCC776` `#C7AD24` |
| `vangogh-straw-hat` | Vincent van Gogh — Self-Portrait with a Straw Hat | `#FBDC30` `#A7A651` `#E0BA7A` `#9BA7B0` `#5A5F80` |
| `vangogh-arles` | Vincent van Gogh — Bedroom in Arles | `#374D8D` `#93A0CB` `#82A866` `#C4B743` `#A35029` |
| `varo-harmony` | Remedios Varo — Harmony | `#C8DAAD` `#989E53` `#738D60` `#DEBC31` `#9D471A` |
| `velazquez-meninas` | Diego Velázquez — Las meninas | `#413A2C` `#241F1A` `#C5B49B` `#A57F5B` `#5C351E` |
| `vermeer-pearl-earring` | Johannes Vermeer — Girl with a Pearl Earring | `#0C0B10` `#707DA6` `#CCAD9D` `#B08E4A` `#863B34` |
| `vermeer-milkmaid` | Johannes Vermeer — The milkmaid | `#022F69` `#D6C17A` `#D8D0BE` `#6B724B` `#7C3E2F` |
| `warhol-flowers` | Andy Warhol — Flowers, 1964 | `#F26386` `#F588AF` `#A4D984` `#FCBC52` `#FD814E` |
| `warhol-marilyn` | Andy Warhol — Marilyn Monroe, 1967 | `#FD0C81` `#FFED4D` `#C34582` `#EBA49E` `#272324` |
| `warhol-liz` | Andy Warhol — Liz | `#D32934` `#2F191B` `#2BAA92` `#D12E6C` `#F4BCB9` |
| `warhol-jagger` | Andy Warhol — Mick Jagger | `#A99364` `#DA95AA` `#F4F0E4` `#B74954` `#C2DDB2` |
| `wood-american-gothic` | Grant Wood — American Gothic | `#A6BDB0` `#8B842F` `#41240B` `#9C4823` `#D6AA7E` |
| `xanto-maiolica` | Francesco Xanto — An Urbino Maiolica Istoriato | `#2C6AA5` `#D9AE2C` `#DDC655` `#D88C27` `#64894D` |
| `youngerman-1953` | Jack Youngerman — Untitled, 1953 | `#59A55D` `#EFDB56` `#7D9DC6` `#ECA23F` `#CA4D2A` |
| `zerbe-harlem` | Karl Zerbe — Harlem | `#46734F` `#CAAB6C` `#D0CCAF` `#617F97` `#9A352D` |
