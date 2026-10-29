// Regression test for the actual sys_top scaler-output block, extracted by test.py.
module scaler_test;
    reg clk_hdmi = 0;
    always #5 clk_hdmi = !clk_hdmi;
    reg ypbpr_en = 0, ilace_auto = 0, yc_en = 0, pal_en = 0, cvbs = 0;
    reg [39:0] PhaseInc = 40'd196787592471;
    reg [16:0] ColorBurst_Range = {7'd20,10'd70};
    reg [23:0] hdmi_data_osd = 0;
    reg hdmi_hs_osd = 0, hdmi_vs_osd = 0, hdmi_cs_osd = 0, hdmi_de_osd = 0;
    wire [23:0] vgas_o, reference_rgb;
    wire vgas_hs, vgas_vs, vgas_cs, vgas_de;
    wire ref_hs, ref_vs, ref_cs, ref_de;
    test_scaler dut(.*);
    vga_out original (
        .clk(clk_hdmi), .ypbpr_en(ypbpr_en),
        .din({24{hdmi_de_osd}} & hdmi_data_osd), .dout(reference_rgb),
        .hsync(hdmi_hs_osd), .vsync(hdmi_vs_osd), .csync(hdmi_cs_osd), .de(hdmi_de_osd),
        .hsync_o(ref_hs), .vsync_o(ref_vs), .csync_o(ref_cs), .de_o(ref_de)
    );
    initial begin
        for (int mode_no=0; mode_no<6; mode_no++) begin
            // RGB, component, progressive Y/C, and interlaced RGB/component
            // must all retain the original scaler samples and latency.
            ypbpr_en = mode_no[0];
            ilace_auto = mode_no>=4;
            yc_en = mode_no==2 || mode_no==3;
            for (int sample_no=0; sample_no<1000; sample_no++) begin
                @(negedge clk_hdmi);
                hdmi_data_osd = 24'($random);
                {hdmi_hs_osd, hdmi_vs_osd, hdmi_cs_osd, hdmi_de_osd} = 4'($random);
                @(posedge clk_hdmi); #1;
                if (sample_no>8 && {vgas_o,vgas_hs,vgas_vs,vgas_cs,vgas_de} !==
                    {reference_rgb,ref_hs,ref_vs,ref_cs,ref_de})
                    $fatal(1,"Existing scaler path changed in mode %d", mode_no);
            end
        end
        ilace_auto = 1; yc_en = 1; ypbpr_en = 0;
        hdmi_data_osd = 24'hff0000;
        hdmi_hs_osd = 1; hdmi_vs_osd = 0; hdmi_cs_osd = 1; hdmi_de_osd = 1;
        repeat(100) @(negedge clk_hdmi);
        hdmi_hs_osd = 0; hdmi_cs_osd = 0;
        for (int composite=0; composite<2; composite++) begin
            cvbs = composite[0];
            repeat(200) @(negedge clk_hdmi);
            repeat(1000) begin
                @(posedge clk_hdmi); #1;
                `ifdef MISTER_DISABLE_YC
                    if (vgas_o !== reference_rgb) $fatal(1,"Disabled Y/C changed RGB");
                `else
                    if (!cvbs && vgas_o[15:8] != 76) $fatal(1,"Interlaced S-Video route not selected");
                    if (cvbs && vgas_o[23:16] != 0) $fatal(1,"Interlaced composite route not selected");
                `endif
            end
        end
        $display("PASS active Y/C routing");
        $display("PASS original RGB/component and non-interlaced scaler paths");
        $finish;
    end
endmodule
